package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"io/ioutil"
	"log"

	"strconv"
	"strings"
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/pelletier/go-toml"
)

const (
	iconCPU        = "\U000F0EE0"
	iconRAM        = "\U000F035B"
	iconUp         = "\U000F0552"
	iconDown       = "\U000F01DA"
	iconPlug       = "\U000F06A5"
	iconPacman     = "\U000F0BAF"
	iconBrightness = "\U000F00E0"
)

var (
	iconTimeArr = [12]string{"\U000F143F", "\U000F1440", "\U000F1441", "\U000F1442", "\U000F1443", "\U000F1444", "\U000F1455", "\U000F1446", "\U000F1447", "\U000F1448", "\U000F1449", "\U000F144A"}
	iconBatArr  = [6]string{"\U000F008E", "\U000F007B", "\U000F007D", "\U000F007F", "\U000F0081", "\U000F0079"}
	iconVolArr  = [4]string{"\U000F075F", "\U000F057F", "\U000F0580", "\U000F057E"}
	netDevMap   = map[string]struct{}{}
	cpuOld, _   = cpu.Get()
	rxOld       = 0
	txOld       = 0
	wlan        = "wlan0"
	lan         = "enp4s0"
	style       = "foreground"
	netColor    = "#d08070"
	cpuColor    = "#fcb103"
	memColor    = "#a3be8c"
	briColor    = "#7ec7a2"
	volColor    = "#789fcc"
	batColor    = "#88c0d0"
	datColor    = "#b48ead"
	duration    = 0
	update      = false
	briefStyle  = "^c"
)

func main() {
	parseConfig()
	for {
		status := setStyle(style)
		s := strings.Join(status, " ")
		exec.Command("xsetroot", "-name", s).Run()

		var now = time.Now()
		time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now))
	}
}

func setStyle(style string) []string {
	return []string{
		updateNet() + updateCPU() + updateMem() + updateBrightness() + updateVolume() + updateBattery() + updateDateTime(),
	}
}

func getNetSpeed() (int, int) {
	dev, err := os.Open("/proc/net/dev")
	if err != nil {
		log.Fatalln(err)
	}
	defer dev.Close()

	devName, rx, tx, rxNow, txNow, void := "", 0, 0, 0, 0, 0
	for scanner := bufio.NewScanner(dev); scanner.Scan(); {
		_, err = fmt.Sscanf(scanner.Text(), "%s %d %d %d %d %d %d %d %d %d", &devName, &rx, &void, &void, &void, &void, &void, &void, &void, &tx)
		if devName == wlan + ":" {
			rxNow += rx
			txNow += tx
		} else if devName == lan + ":" {
		    rxNow += rx
		    txNow += tx
		}

	}
	return rxNow, txNow
}

func updateNet() string {
	rxNow, txNow := getNetSpeed()
	defer func() { rxOld, txOld = rxNow, txNow }()
	return briefStyle + netColor + "^" + iconDown + fmtNetSpeed(float64(rxNow-rxOld)) + " " + iconUp + fmtNetSpeed(float64(txNow-txOld)) + " "

}

func fmtNetSpeed(speed float64) string {
	if speed < 0 {
		log.Fatalln("Speed must be positive")
	}
	var res string

	switch {
	case speed >= (1024 * 1024 * 1024):
		gbSpeed := speed / (1024.0 * 1024.0 * 1024.0)
		res = fmt.Sprintf("%.1f", gbSpeed) + "GB"
	case speed >= (1024 * 1024):
		mbSpeed := speed / (1024.0 * 1024.0)
		res = fmt.Sprintf("%.1f", mbSpeed) + "MB"
	case speed >= 1024:
		kbSpeed := speed / 1024.0
		res = fmt.Sprintf("%.1f", kbSpeed) + "KB"
	case speed >= 0:
		res = fmt.Sprint(speed) + "B"
	}

	return res
}

func updateMem() string {
	meminfo, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Fatalln(err)
	}
	defer meminfo.Close()

	var total, avail float64
	for info := bufio.NewScanner(meminfo); info.Scan(); {
		key, value := "", 0.0
		if _, err = fmt.Sscanf(info.Text(), "%s %f", &key, &value); err != nil {
			log.Fatalln(err)
		}
		if key == "MemTotal:" || key == "SwapTotal:" {
			total += value
		}
		if key == "MemAvailable:" || key == "SwapFree:" {
			avail += value
		}
	}
	used := (total - avail) / 1024.0 / 1024.0

	return briefStyle + memColor + "^" + iconRAM + " " + fmt.Sprintf("%.2f", used) + "GiB "
}

func updateCPU() string {
	cpuNow, err := cpu.Get()
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer func() { cpuOld = cpuNow }()
	total := float64(cpuNow.Total - cpuOld.Total)
	usage := 100.0 - float64(cpuNow.Idle-cpuOld.Idle)/total*100
	return briefStyle + cpuColor + "^" + iconCPU + " " + fmt.Sprintf("%.2f", usage) + "% "
}

func updateVolume() string {
	const pamixer = "pamixer"
	isMuted, _ := strconv.ParseBool(cmdReturn(pamixer, "--get-mute", false))
	volume := cmdReturn(pamixer, "--get-volume", true)
	if isMuted {
		return briefStyle + volColor + "^" + iconVolArr[0] + " "
	} else {
		return briefStyle + volColor + "^" + getVolIcon(volume) + " " + volume + " "
	}
}

func updateBrightness() string {
	const brightnessctl = "brightnessctl"
	brightness := cmdReturn(brightnessctl, "get", true)
	bright, _ := strconv.ParseInt(brightness, 10, 64)
	if bright == 120000 {
		return ""
	} else {
		return briefStyle + briColor + "^" + iconBrightness + " " + strconv.FormatInt(bright/1200, 10) + " "
	}
}

func getVolIcon(volume string) string {
	var res string
	volumeInt, _ := strconv.ParseInt(volume, 10, 32)
	if volumeInt > 65 {
		res = iconVolArr[3]
	} else if volumeInt > 30 {
		res = iconVolArr[2]
	} else {
		res = iconVolArr[1]
	}
	return res
}

func cmdReturn(bin string, arg string, output bool) string {
	var res string
	cmd := exec.Command(bin, arg)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if output {
			log.Println(err)
		}
	}
	res = strings.TrimSpace(string(stdout.Bytes()))

	return res
}

func updateBattery() string {
	const pathToPowerSupply = "/sys/class/power_supply/"
	var pathToBat0 = pathToPowerSupply + "BAT0/"
	var pathToAC = pathToPowerSupply + "AC0/"

	status := parseTxt(pathToBat0, "status")
	capacity := parseTxt(pathToBat0, "capacity")
	isPlugged, _ := strconv.ParseBool(parseTxt(pathToAC, "online"))
	if status == "Full" {
		// return iconBatArr[5] + " Full"
		return ""
	} else if isPlugged == true {
		return briefStyle + batColor + "^" + iconPlug + " " + capacity + " "
	} else {
		return briefStyle + batColor + "^" + getBatIcon(capacity) + " " + capacity + " "
	}
}

func getBatIcon(capacity string) string {
	var res string
	capacityInt, _ := strconv.ParseInt(capacity, 10, 32)
	if capacityInt == 100 {
		res = iconBatArr[5]
	} else if capacityInt >= 80 {
		res = iconBatArr[4]
	} else if capacityInt > 60 {
		res = iconBatArr[3]
	} else if capacityInt > 40 {
		res = iconBatArr[2]
	} else if capacityInt > 20 {
		res = iconBatArr[1]
	} else {
		res = iconBatArr[0]
	}
	return res
}

func parseTxt(path string, name string) string {
	var res string
	contentOri, err := ioutil.ReadFile(path + name)
	if err != nil {
		log.Println("Please check the " + name + "'s path")
	}
	res = strings.TrimSpace(string(contentOri))

	return res
}

func parseConfig() {
	dirname, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}
	path := dirname + "/.config/goblocks/config.toml"

	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	config, _ := toml.Load(string(content))
	// wlan = config.Get("networks.wlan").(string) + ":"
	// lan = config.Get("networks.lan").(string) + ":"

	style = config.Get("color.style").(string)

	netDevMap[wlan] = struct{}{}
	netDevMap[lan] = struct{}{}
}

func updateDateTime() string {
	var hour = time.Now().Hour()
	var dateTime = time.Now().Local().Format("2006-01-02 Mon 15:04:05 ")

	return briefStyle + datColor + "^" + getHourIcon(hour) + " " + dateTime
}

func getHourIcon(hour int) string {
	var res string
	if hour == 0 || hour == 12 {
		res = iconTimeArr[11]
	} else if hour == 23 || hour == 11 {
		res = iconTimeArr[10]
	} else if hour == 22 || hour == 10 {
		res = iconTimeArr[9]
	} else if hour == 21 || hour == 9 {
		res = iconTimeArr[8]
	} else if hour == 20 || hour == 8 {
		res = iconTimeArr[7]
	} else if hour == 19 || hour == 7 {
		res = iconTimeArr[6]
	} else if hour == 18 || hour == 6 {
		res = iconTimeArr[5]
	} else if hour == 17 || hour == 5 {
		res = iconTimeArr[4]
	} else if hour == 16 || hour == 4 {
		res = iconTimeArr[3]
	} else if hour == 15 || hour == 3 {
		res = iconTimeArr[2]
	} else if hour == 14 || hour == 2 {
		res = iconTimeArr[1]
	} else if hour == 13 || hour == 1 {
		res = iconTimeArr[0]
	}
	return res
}
