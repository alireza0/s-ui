package service

import (
	"sort"
	"sync"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type onlines struct {
	Inbound  []string `json:"inbound,omitempty"`
	User     []string `json:"user,omitempty"`
	Outbound []string `json:"outbound,omitempty"`
}

var (
	// Guards both values below: SaveStats runs on the ten-second cron while
	// GetOnlines is read from a gin handler.
	statsMu         sync.Mutex
	onlineResources = &onlines{}

	// Traffic drained from the core that has not reached the database yet.
	// GetStats is destructive (it Swap(0)s every counter), so without this a
	// single SQLITE_BUSY loses the whole ten-second window for every user.
	pendingStats []model.Stats
)

type StatsService struct {
}

func (s *StatsService) SaveStats(enableTraffic bool, bucketSeconds int64) error {
	if corePtr == nil || !corePtr.IsRunning() {
		return nil
	}
	box := corePtr.GetInstance()
	if box == nil {
		return nil
	}
	st := box.SessionTracker()
	if st == nil {
		return nil
	}
	drained := st.GetStats()

	statsMu.Lock()
	// Anything a previous cycle could not commit goes in ahead of this one.
	batch := append(pendingStats, (*drained)...)
	pendingStats = nil
	online := &onlines{}
	statsMu.Unlock()

	if len(batch) == 0 {
		statsMu.Lock()
		onlineResources = online
		statsMu.Unlock()
		return nil
	}

	var err error
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			if cErr := tx.Commit().Error; cErr != nil {
				err = cErr
			}
		} else {
			tx.Rollback()
		}
		statsMu.Lock()
		if err != nil {
			// Hold the drained traffic for the next cycle rather than dropping
			// it on the floor.
			pendingStats = batch
		} else {
			onlineResources = online
		}
		statsMu.Unlock()
	}()

	now := time.Now().Unix()

	// Aggregate per-resource so each active inbound/outbound/user is reported
	// online once (a tag may now appear in both directions), and each user's
	// up+down collapse into a single UPDATE.
	type traffic struct{ up, down int64 }
	userTraffic := map[string]*traffic{}
	seenInbound := map[string]bool{}
	seenOutbound := map[string]bool{}
	for _, stat := range batch {
		switch stat.Resource {
		case "inbound":
			if !seenInbound[stat.Tag] {
				seenInbound[stat.Tag] = true
				online.Inbound = append(online.Inbound, stat.Tag)
			}
		case "outbound":
			if !seenOutbound[stat.Tag] {
				seenOutbound[stat.Tag] = true
				online.Outbound = append(online.Outbound, stat.Tag)
			}
		case "user":
			t, ok := userTraffic[stat.Tag]
			if !ok {
				t = &traffic{}
				userTraffic[stat.Tag] = t
				online.User = append(online.User, stat.Tag)
			}
			if stat.Direction {
				t.up += stat.Traffic
			} else {
				t.down += stat.Traffic
			}
		}
	}

	for name, t := range userTraffic {
		update := map[string]interface{}{"online_at": now}
		if t.up > 0 {
			update["up"] = gorm.Expr("up + ?", t.up)
		}
		if t.down > 0 {
			update["down"] = gorm.Expr("down + ?", t.down)
		}
		err = tx.Model(model.Client{}).Where("name = ?", name).Updates(update).Error
		if err != nil {
			return err
		}
	}

	if !enableTraffic {
		return nil
	}

	// Round each sample down to its bucket and upsert, so all 10s cycles within
	// the same bucket accumulate into one row per (resource, tag, direction).
	if bucketSeconds < 1 {
		bucketSeconds = 1
	}
	bucket := now - (now % bucketSeconds)
	for i := range batch {
		batch[i].DateTime = bucket
	}
	err = tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "resource"}, {Name: "tag"}, {Name: "date_time"}, {Name: "direction"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"traffic": gorm.Expr("stats.traffic + excluded.traffic")}),
	}).Create(&batch).Error
	return err
}

func (s *StatsService) GetStats(resource string, tag string, limit int, start int64, end int64) (any, error) {
	var err error
	var result []model.Stats

	// Custom range when both start and end are provided, otherwise the last
	// `limit` hours up to now.
	var startTime, endTime int64
	if start > 0 && end > start {
		startTime, endTime = start, end
	} else {
		endTime = time.Now().Unix()
		startTime = endTime - (int64(limit) * 3600)
	}

	db := database.GetDB()
	resources := []string{resource}
	if resource == "endpoint" {
		resources = []string{"inbound", "outbound"}
	}
	err = db.Model(model.Stats{}).Where("resource in ? AND tag = ? AND date_time > ? AND date_time <= ?", resources, tag, startTime, endTime).Order("date_time ASC").Scan(&result).Error
	if err != nil {
		return nil, err
	}

	bucketSeconds, _ := (&SettingService{}).GetStatsBucketSeconds()
	if bucketSeconds < 1 {
		bucketSeconds = 1
	}
	numBuckets := 360
	if maxBuckets := (endTime - startTime) / bucketSeconds; maxBuckets < int64(numBuckets) {
		numBuckets = int(maxBuckets)
	}
	if numBuckets < 1 {
		numBuckets = 1
	}

	return s.downsampleStats(result, startTime, endTime, numBuckets), nil
}

func (s *StatsService) downsampleStats(stats []model.Stats, startTime, endTime int64, numBuckets int) any {
	result := make(map[int64][]int64)
	bucketSpan := (endTime - startTime) / int64(numBuckets)
	if bucketSpan == 0 {
		bucketSpan = 1
	}

	for _, r := range stats {
		bucket := (r.DateTime - startTime) / bucketSpan
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= int64(numBuckets) {
			bucket = int64(numBuckets) - 1
		}
		if _, ok := result[bucket]; !ok {
			result[bucket] = []int64{0, 0}
		}
		if r.Direction {
			result[bucket][0] += r.Traffic
		} else {
			result[bucket][1] += r.Traffic
		}
	}

	return map[string]any{"stats": result, "startTime": startTime, "bucketSpan": bucketSpan, "numBuckets": numBuckets}
}

func (s *StatsService) GetOnlines() (onlines, error) {
	statsMu.Lock()
	defer statsMu.Unlock()
	// Copied: the caller must not hold slices the next cron tick replaces.
	return onlines{
		Inbound:  append([]string(nil), onlineResources.Inbound...),
		User:     append([]string(nil), onlineResources.User...),
		Outbound: append([]string(nil), onlineResources.Outbound...),
	}, nil
}

// GetSessions lists the live routed connections, narrowed to one user, inbound
// or outbound. Sorted newest first so the panel shows fresh connections on top.
func (s *StatsService) GetSessions(resource string, tag string) ([]core.SessionInfo, error) {
	if corePtr == nil || !corePtr.IsRunning() {
		return []core.SessionInfo{}, nil
	}
	box := corePtr.GetInstance()
	if box == nil {
		return []core.SessionInfo{}, nil
	}
	sessions := box.SessionTracker().Sessions()
	if tag != "" {
		var match func(core.SessionInfo) bool
		switch resource {
		case "user":
			match = func(session core.SessionInfo) bool { return session.User == tag }
		case "inbound":
			match = func(session core.SessionInfo) bool { return session.Inbound == tag }
		case "outbound":
			match = func(session core.SessionInfo) bool { return session.Outbound == tag }
		case "endpoint":
			// An endpoint can serve either side, so it is matched on both.
			match = func(session core.SessionInfo) bool {
				return session.Inbound == tag || session.Outbound == tag
			}
		default:
			return nil, common.NewError("unknown resource: ", resource)
		}
		filtered := sessions[:0]
		for _, session := range sessions {
			if match(session) {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt > sessions[j].CreatedAt
	})
	return sessions, nil
}

// CloseUserSessions disconnects a user: every routed connection of theirs is
// closed, and so is every protocol-level session that has one of its own, which
// is what a multiplex or QUIC client would otherwise keep using. The user stays
// enabled, so nothing stops them from connecting again.
func (s *StatsService) CloseUserSessions(user string) error {
	if user == "" {
		return common.NewError("empty user name")
	}
	if corePtr == nil || !corePtr.IsRunning() {
		return common.NewError("core is not running")
	}
	box := corePtr.GetInstance()
	if box == nil {
		return common.NewError("core is not running")
	}
	box.SessionTracker().CloseByUser(user)
	corePtr.KickUserSessions(user)
	return nil
}

// delOldStatsChunk caps how many rows one DELETE removes, so the write lock is
// released between chunks.
const delOldStatsChunk = 5000

// DelOldStats drops stats older than the retention window, in bounded chunks.
// One unbounded DELETE held the write lock past the busy timeout, which made
// the daily cleanup itself a cause of lost traffic accounting.
func (s *StatsService) DelOldStats(days int) error {
	oldTime := time.Now().AddDate(0, 0, -(days)).Unix()
	db := database.GetDB()
	for {
		res := db.Where("id IN (?)",
			db.Model(model.Stats{}).Select("id").Where("date_time < ?", oldTime).Limit(delOldStatsChunk),
		).Delete(model.Stats{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected < delOldStatsChunk {
			return nil
		}
	}
}
