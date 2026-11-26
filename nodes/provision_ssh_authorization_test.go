package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestLiveSSHConcurrentAuthorizationsPreserveOtherKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux SSH installer")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("requires python3")
	}
	home := t.TempDir()
	folder := filepath.Join(home, ".ssh")
	if err = os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	existing := "ssh-ed25519 existing-unmanaged-key\n"
	if err = os.WriteFile(filepath.Join(folder, "authorized_keys"), []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}
	header := `import json,os,sys
from pathlib import Path
p=json.load(sys.stdin);slot=str(p['id'])+'-'+p['owner'][:12]
`
	script := strings.Replace(liveAuthorizeKeyScript, liveHostHeader, header, 1)
	script = strings.Replace(script, "account=pwd.getpwnam(p['ssh_user'])", "account=__import__('types').SimpleNamespace(pw_dir=p['test_home'],pw_uid=os.getuid(),pw_gid=os.getgid())", 1)
	var wait sync.WaitGroup
	failures := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			body, _ := json.Marshal(map[string]interface{}{"id": i + 1, "owner": fmt.Sprintf("%032x", i+1), "test_home": home, "ssh_user": "fixture", "control_port": 16500 + i, "authorized_key": fmt.Sprintf("ssh-ed25519 fixture-key-%d", i)})
			cmd := exec.Command(python, "-c", script)
			cmd.Stdin = bytes.NewReader(body)
			if output, err := cmd.CombinedOutput(); err != nil {
				failures <- fmt.Errorf("authorization: %w: %s", err, output)
			}
		}(i)
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(folder, "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), existing) || len(strings.Split(strings.TrimSpace(string(body)), "\n")) != 21 {
		t.Fatal("concurrent authorizations lost an existing key")
	}
	for i := 0; i < 20; i++ {
		if !strings.Contains(string(body), fmt.Sprintf("ssh-ed25519 fixture-key-%d s-ui-managed-node-%d-", i, i+1)) {
			t.Fatal("managed key missing")
		}
	}
}
