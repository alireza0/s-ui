package service

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"s-ui/config"
	"s-ui/logger"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type ServerService struct {
	SingBoxService
}

func (s *ServerService) GetStatus(request string) *map[string]interface{} {
	status := make(map[string]interface{}, 0)
	requests := strings.Split(request, ",")
	for _, req := range requests {
		switch req {
		case "cpu":
			status["cpu"] = s.GetCpuPercent()
		case "mem":
			status["mem"] = s.GetMemInfo()
		case "net":
			status["net"] = s.GetNetInfo()
		case "sys":
			status["uptime"] = s.GetUptime()
			status["sys"] = s.GetSystemInfo()
		case "sbd":
			status["sbd"] = s.GetSingboxInfo()
		}
	}
	return &status
}

func (s *ServerService) GetCpuPercent() float64 {
	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
		return 0
	} else {
		return percents[0]
	}
}

func (s *ServerService) GetUptime() uint64 {
	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
		return 0
	} else {
		return upTime
	}
}

func (s *ServerService) GetMemInfo() map[string]interface{} {
	info := make(map[string]interface{}, 0)
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		info["current"] = memInfo.Used
		info["total"] = memInfo.Total
	}
	return info
}

func (s *ServerService) GetNetInfo() map[string]interface{} {
	info := make(map[string]interface{}, 0)
	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		info["sent"] = ioStat.BytesSent
		info["recv"] = ioStat.BytesRecv
		info["psent"] = ioStat.PacketsSent
		info["precv"] = ioStat.PacketsRecv
	} else {
		logger.Warning("can not find io counters")
	}
	return info
}

func (s *ServerService) GetSingboxInfo() map[string]interface{} {
	info := make(map[string]interface{}, 0)
	sysStats, err := s.SingBoxService.GetSysStats()
	if err == nil {
		info["running"] = true
		info["stats"] = sysStats
	} else {
		info["running"] = s.SingBoxService.IsRunning()
	}
	return info
}

func (s *ServerService) GetSystemInfo() map[string]interface{} {
	info := make(map[string]interface{}, 0)
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)

	info["appMem"] = rtm.Sys
	info["appThreads"] = uint32(runtime.NumGoroutine())
	cpuInfo, err := cpu.Info()
	if err == nil {
		info["cpuType"] = cpuInfo[0].ModelName
	}
	info["cpuCount"] = runtime.NumCPU()
	info["hostName"], _ = os.Hostname()
	info["appVersion"] = config.GetVersion()
	ipv4 := make([]string, 0)
	ipv6 := make([]string, 0)
	// get ip address
	netInterfaces, _ := net.Interfaces()
	for i := 0; i < len(netInterfaces); i++ {
		if len(netInterfaces[i].Flags) > 2 && netInterfaces[i].Flags[0] == "up" && netInterfaces[i].Flags[1] != "loopback" {
			addrs := netInterfaces[i].Addrs

			for _, address := range addrs {
				if strings.Contains(address.Addr, ".") {
					ipv4 = append(ipv4, address.Addr)
				} else if address.Addr[0:6] != "fe80::" {
					ipv6 = append(ipv6, address.Addr)
				}
			}
		}
	}
	info["ipv4"] = ipv4
	info["ipv6"] = ipv6

	return info
}

func (s *ServerService) GetLogs(service string, count string, level string) []string {
	c, _ := strconv.Atoi(count)

	if service == "s-ui" {
		return logger.GetLogs(c, level)
	}
	ppid := os.Getppid()
	var lines []string
	var cmdArgs []string
	if ppid > 1 {
		cmdArgs = []string{"journalctl", "-u", service, "--no-pager", "-n", count, "-p", level}
	} else {
		cmdArgs = []string{"tail", "/logs/" + service + ".log", "-n", count}
	}
	// Run the command
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return []string{"Failed to get logs!", err.Error()}
	}
	lines = strings.Split(out.String(), "\n")

	return lines
}

func (s *ServerService) GenKeypair(keyType string, options string) []string {
	if len(keyType) == 0 {
		return []string{"No keypair to generate"}
	}
	sbExec := s.GetBinaryPath()
	cmdArgs := []string{"generate", keyType + "-keypair"}
	if keyType == "tls" || keyType == "ech" {
		cmdArgs = append(cmdArgs, options)
	}
	// Run the command
	cmd := exec.Command(sbExec, cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return []string{"Failed to generate keypair"}
	}
	return strings.Split(out.String(), "\n")
}
