package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var resourceLocks sync.Map

const dnsRecheckGrace = 2 * time.Minute

func resourceLock(id uint) *sync.Mutex {
	value, _ := resourceLocks.LoadOrStore(id, &sync.Mutex{})
	return value.(*sync.Mutex)
}
func leaseResources(id uint) (func(error), bool, error) {
	db := database.GetDB()
	noop := func(error) { /* No lease was acquired, so there is nothing to release. */ }
	if err := db.Transaction(func(tx *gorm.DB) error {
		var node model.Node
		if err := tx.First(&node, id).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "node_id"}, {Name: "kind"}}, DoNothing: true}).Create(&model.NodeTask{NodeId: id, Kind: "resources", Generation: 1, State: "queued"}).Error
	}); err != nil {
		return noop, false, err
	}
	var task model.NodeTask
	if err := db.First(&task, "node_id = ? AND kind = ?", id, "resources").Error; err != nil {
		return noop, false, err
	}
	if task.NextAttempt > time.Now().Unix() {
		return noop, false, nil
	}
	uid, err := uuid.NewV4()
	if err != nil {
		return noop, false, err
	}
	token := uid.String()
	result := db.Model(&model.NodeTask{}).Where("id = ? AND generation = ? AND lease_until < ?", task.Id, task.Generation, time.Now().Unix()).Updates(map[string]interface{}{"lease_token": token, "lease_until": time.Now().Unix() + 600})
	if result.Error != nil || result.RowsAffected == 0 {
		return noop, false, result.Error
	}
	return func(failure error) {
		_ = db.Transaction(func(tx *gorm.DB) error {
			var current model.NodeTask
			if err := tx.First(&current, "id = ? AND lease_token = ?", task.Id, token).Error; err != nil {
				return err
			}
			updates := map[string]interface{}{"lease_token": "", "lease_until": 0, "state": "queued", "next_attempt": 0}
			if current.Generation != task.Generation {
				return tx.Model(&current).Updates(updates).Error
			}
			if failure == nil {
				updates["state"], updates["attempts"], updates["last_error"] = "done", 0, ""
			} else {
				updates["attempts"], updates["last_error"] = current.Attempts+1, safeResourceError(failure)
				updates["next_attempt"] = time.Now().Unix() + (int64(1) << min(current.Attempts, 6)) + int64(rand.IntN(3))
			}
			return tx.Model(&current).Updates(updates).Error
		})
	}, true, nil
}

func ReconcileResources(parent context.Context, id uint) (failure error) {
	lock := resourceLock(id)
	lock.Lock()
	defer lock.Unlock()
	db := database.GetDB()
	node, err := resourceNode(db, id)
	if err != nil {
		return err
	}
	if !node.Enable {
		return nil
	}
	finish, run, err := leaseResources(id)
	if err != nil || !run {
		return err
	}
	defer func() { finish(failure) }()
	ctx, cancel := context.WithTimeout(parent, 6*time.Minute)
	defer cancel()
	var domain model.NodeDomainBinding
	if err := db.First(&domain, whereNodeID, id).Error; err != nil {
		return err
	}
	phase := "domain"
	defer func() { recordResourceFailure(db, node, domain, phase, failure) }()
	if err := prepareResourceDomain(ctx, db, id, domain); err != nil {
		return err
	}
	if err := verifyDomainAddress(ctx, domain.FQDN, domain.ExpectedIP); err != nil {
		return err
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return acknowledgeDomain(tx, id, domain) }); err != nil {
		return err
	}
	phase = "certificate"
	if err := db.First(&node, id).Error; err != nil {
		return err
	}
	return reconcileCertificate(ctx, node, domain)
}
func netIPIsV4(value string) bool { ip := net.ParseIP(value); return ip != nil && ip.To4() != nil }

func resourceLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		var bindings []model.NodeDomainBinding
		if database.GetDB().Find(&bindings).Error == nil {
			limit := make(chan struct{}, 2)
			var wait sync.WaitGroup
			for _, binding := range bindings {
				if ctx.Err() != nil {
					break
				}
				limit <- struct{}{}
				wait.Add(1)
				go func(id uint) { defer wait.Done(); defer func() { <-limit }(); _ = ReconcileResources(ctx, id) }(binding.NodeId)
			}
			wait.Wait()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func RetryNodeResources(id uint, actor string) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if _, err := resourceNode(tx, id); err != nil {
			return err
		}
		if err := database.QueueNodeTasks(tx, []uint{id}, "resources"); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]uint{"id": id})
		return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: "node-resources", Action: "retry", Obj: body}).Error
	})
}
func CleanupNodeDNS(ctx context.Context, id, recordID uint, actor string) error {
	lock := resourceLock(id)
	lock.Lock()
	defer lock.Unlock()
	db := database.GetDB()
	var node model.Node
	if err := db.First(&node, id).Error; err != nil {
		return err
	}
	var task model.NodeTask
	if err := db.First(&task, "node_id = ? AND kind = ?", id, "resources").Error; err == nil && task.LeaseUntil > time.Now().Unix() {
		return errors.New("node_resource_job_running")
	}
	var domain model.NodeDomainBinding
	db.First(&domain, whereNodeID, id)
	var records []model.NodeDNSRecord
	query := db.Where("node_id = ? AND retired = ?", id, false)
	if recordID != 0 {
		query = query.Where("id = ?", recordID)
	}
	if err := query.Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		if node.Enable && !record.Challenge && domain.Mode == "managed" && record.Name == domain.FQDN {
			return errors.New("disable_node_before_dns_cleanup")
		}
		if err := removeResourceRecord(ctx, record); err != nil {
			return err
		}
	}
	body, _ := json.Marshal(map[string]uint{"node_id": id, "record_id": recordID})
	return db.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: actor, Key: "node-dns", Action: "cleanup", Obj: body}).Error
}

func recordResourceFailure(db *gorm.DB, node model.Node, domain model.NodeDomainBinding, phase string, failure error) {
	if failure == nil || errors.Is(failure, context.Canceled) || safeResourceError(failure) == "certificate_target_changed" {
		return
	}
	code := safeResourceError(failure)
	_ = db.Transaction(func(tx *gorm.DB) error {
		var current model.Node
		if err := tx.First(&current, node.Id).Error; err != nil {
			return err
		}
		if current.ManagementGeneration != node.ManagementGeneration || current.ControlEpoch != node.ControlEpoch {
			return nil
		}
		if phase == "domain" {
			// Keep a recently verified name usable across transient DNS failures.
			// Changed targets and answers pointing to a different address must fail.
			if (errors.Is(failure, errDNSLookupFailed) || errors.Is(failure, errDNSAddressNotResolved)) &&
				domain.State == "ready" && domain.AppliedFQDN == domain.FQDN &&
				time.Since(time.Unix(domain.CheckedAt, 0)) < dnsRecheckGrace {
				return nil
			}
			return tx.Model(&model.NodeDomainBinding{}).Where("node_id = ? AND generation = ?", node.Id, domain.Generation).Updates(map[string]interface{}{"state": "failed", "last_error": code, "checked_at": time.Now().Unix()}).Error
		}
		return tx.Model(&model.NodeCertificateBinding{}).Where(whereNodeID, node.Id).Update("last_error", code).Error
	})
}

func prepareResourceDomain(ctx context.Context, db *gorm.DB, id uint, domain model.NodeDomainBinding) error {
	if domain.Mode == "managed" {
		var profile model.NodeDNSProfile
		if err := db.First(&profile, domain.ProfileId).Error; err != nil {
			return err
		}
		kind := "AAAA"
		if netIPIsV4(domain.ExpectedIP) {
			kind = "A"
		}
		if _, err := ensureResourceRecord(ctx, id, profile, domain.ZoneId, domain.FQDN, kind, domain.ExpectedIP); err != nil {
			return err
		}
	}
	return nil
}

func acknowledgeDomain(tx *gorm.DB, id uint, domain model.NodeDomainBinding) error {
	var current model.NodeDomainBinding
	if err := tx.First(&current, whereNodeID, id).Error; err != nil {
		return err
	}
	if current.Generation != domain.Generation {
		return errors.New("domain_target_changed")
	}
	changed := current.State != "ready" || current.AppliedFQDN != domain.FQDN
	if err := tx.Model(&current).Updates(map[string]interface{}{"state": "ready", "last_error": "", "checked_at": time.Now().Unix(), "applied_fqdn": domain.FQDN}).Error; err != nil {
		return err
	}
	var target model.Node
	if err := tx.First(&target, id).Error; err != nil {
		return err
	}
	if target.LinkAddress != domain.FQDN {
		changed = true
		if err := tx.Model(&target).Update("link_address", domain.FQDN).Error; err != nil {
			return err
		}
		target.LinkAddress = domain.FQDN
	}
	if changed {
		return queueNodeStructure(tx, &target)
	}
	return nil
}
