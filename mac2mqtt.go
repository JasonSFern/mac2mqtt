package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var hostname string

type config struct {
	Ip       string `yaml:"mqtt_ip"`
	Port     string `yaml:"mqtt_port"`
	User     string `yaml:"mqtt_user"`
	Password string `yaml:"mqtt_password"`
	Hostname string `yaml:"hostname"`
}

func (c *config) getConfig() *config {

	configContent, err := os.ReadFile("mac2mqtt.yaml")
	if err != nil {
		log.Fatal("No config file provided")
	}

	err = yaml.Unmarshal(configContent, c)
	if err != nil {
		log.Fatal("No data in config file")
	}

	if c.Ip == "" {
		log.Fatal("Must specify mqtt_ip in mac2mqtt.yaml")
	}

	if c.Port == "" {
		log.Fatal("Must specify mqtt_port in mac2mqtt.yaml")
	}

	if c.User == "" {
		log.Fatal("Must specify mqtt_user in mac2mqtt.yaml")
	}

	if c.Password == "" {
		log.Fatal("Must specify mqtt_password in mac2mqtt.yaml")
	}
	if c.Hostname == "" {
		log.Fatal("must specify a hostname in mac2mqtt.yaml")
	}

	return c
}

func getDeviceSerialnumber() string {
	cmd := "/usr/sbin/ioreg -l | /usr/bin/grep IOPlatformSerialNumber"
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()

	if err != nil {
		log.Fatal(err)
	}

	outputStr := string(output)
	last := output[strings.LastIndex(outputStr, " ")+1:]
	lastStr := string(last)

	// remove all symbols, but [a-zA-Z0-9_-]
	reg, err := regexp.Compile("[^a-zA-Z0-9_-]+")

	if err != nil {
		log.Fatal(err)
	}

	lastStr = reg.ReplaceAllString(lastStr, "")

	return lastStr
}

func getDeviceModel() string {
	cmd := "/usr/sbin/system_profiler SPHardwareDataType |/usr/bin/grep Chip | /usr/bin/sed 's/\\(^.*: \\)\\(.*\\)/\\2/'"
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()

	if err != nil {
		log.Fatal(err)
	}
	outputStr := string(output)
	outputStr = strings.TrimSuffix(outputStr, "\n")
	return outputStr
}

func getHostname() string {
	hostname, err := os.Hostname()

	if err != nil {
		log.Fatal(err)
	}

	// "name.local" => "name"
	firstPart := strings.Split(hostname, ".")[0]

	// remove all symbols, but [a-zA-Z0-9_-]
	reg, err := regexp.Compile("[^a-zA-Z0-9_-]+")
	if err != nil {
		log.Fatal(err)
	}
	firstPart = reg.ReplaceAllString(firstPart, "")

	return firstPart
}

func getCommandOutput(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)

	stdout, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	stdoutStr := string(stdout)
	stdoutStr = strings.TrimSuffix(stdoutStr, "\n")

	return stdoutStr
}

func getMuteStatus() bool {
	output := getCommandOutput("/usr/bin/osascript", "-e", "output muted of (get volume settings)")

	b, err := strconv.ParseBool(output)
	if err != nil {
	}

	return b
}

func getCurrentVolume() int {
	output := getCommandOutput("/usr/bin/osascript", "-e", "output volume of (get volume settings)")

	i, err := strconv.Atoi(output)
	if err != nil {
	}

	return i
}

// Get current brightness level (0-100)
func getCurrentBrightness() int {
	output := getCommandOutput("brightness", "-l")
	
	// Parse the output to extract the brightness value
	// Example output: "brightness 0.50"
	parts := strings.Fields(output)
	if len(parts) < 2 {
		return 50 // Default in case of parsing error
	}
	
	// Find the brightness value in the output
	var brightnessFloat float64
	var err error
	for i, part := range parts {
		if part == "brightness" && i+1 < len(parts) {
			brightnessFloat, err = strconv.ParseFloat(parts[i+1], 64)
			if err != nil {
				log.Printf("Error parsing brightness value: %v\n", err)
				return 50 // Default in case of error
			}
			break
		}
	}
	
	// Convert from 0-1 scale to 0-100
	return int(brightnessFloat * 100)
}

func runCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)

	_, err := cmd.Output()
	if err != nil {
		log.Printf("Error running command %s: %v", name, err)
	}
}

// from 0 to 100
func setVolume(i int) {
	runCommand("/usr/bin/osascript", "-e", "set volume output volume "+strconv.Itoa(i))
}

// from 0 to 100, but brightness tool accepts 0 to 1
func setBrightness(i int) {
	// Convert from 0-100 scale to 0-1 scale
	brightnessValue := float64(i) / 100.0
	
	// Format to 2 decimal places
	brightnessStr := strconv.FormatFloat(brightnessValue, 'f', 2, 64)
	
	// Use the brightness command-line tool
	runCommand("brightness", brightnessStr)
}

// true - turn mute on
// false - turn mute off
func setMute(b bool) {
	runCommand("/usr/bin/osascript", "-e", "set volume output muted "+strconv.FormatBool(b))
}

func commandSleep() {
	runCommand("pmset", "sleepnow")
}

func commandDisplaySleep() {
	runCommand("pmset", "displaysleepnow")
}

func commandShutdown() {

	if os.Getuid() == 0 {
		// if the program is run by root user we are doing the most powerfull shutdown - that always shuts down the computer
		runCommand("shutdown", "-h", "now")
	} else {
		// if the program is run by ordinary user we are trying to shutdown, but it may fail if the other user is logged in
		runCommand("/usr/bin/osascript", "-e", "tell app \"System Events\" to shut down")
	}

}

func commandDisplayWake() {
	runCommand("/usr/bin/caffeinate", "-u", "-t", "1")
}

func commandRunShortcut(shortcut string) {
	runCommand("shortcuts", "run", shortcut)
}

func commandScreensaver() {
	runCommand("open", "-a", "ScreenSaverEngine")
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected to MQTT")

	token := client.Publish(getTopicPrefix()+"/status/alive", 0, true, "true")
	token.Wait()

	log.Println("Sending 'true' to topic: " + getTopicPrefix() + "/status/alive")

	listen(client, getTopicPrefix()+"/command/#")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Disconnected from MQTT: %v", err)
}

func getMQTTClient(ip, port, user, password string) mqtt.Client {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", ip, port))
	opts.SetUsername(user)
	opts.SetPassword(password)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	opts.SetWill(getTopicPrefix()+"/status/alive", "false", 0, true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	return client
}

func getTopicPrefix() string {
	return "mac2mqtt/" + hostname
}

func listen(client mqtt.Client, topic string) {

	token := client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {

		if msg.Topic() == getTopicPrefix()+"/command/volume" {

			i, err := strconv.Atoi(string(msg.Payload()))
			if err == nil && i >= 0 && i <= 100 {

				setVolume(i)

				updateVolume(client)
				updateMute(client)

			} else {
				log.Println("Incorrect value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/brightness" {

			i, err := strconv.Atoi(string(msg.Payload()))
			if err == nil && i >= 0 && i <= 100 {

				setBrightness(i)

				updateBrightness(client)

			} else {
				log.Println("Incorrect brightness value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/mute" {

			b, err := strconv.ParseBool(string(msg.Payload()))
			if err == nil {
				setMute(b)

				updateVolume(client)
				updateMute(client)

			} else {
				log.Println("Incorrect value")
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/set" {

			if string(msg.Payload()) == "sleep" {
				commandSleep()
			}

			if string(msg.Payload()) == "displaysleep" {
				commandDisplaySleep()
			}

			if string(msg.Payload()) == "displaywake" {
				commandDisplayWake()
			}

			if string(msg.Payload()) == "shutdown" {
				commandShutdown()
			}

			if string(msg.Payload()) == "screensaver" {
				commandScreensaver()
			}

		}

		if msg.Topic() == getTopicPrefix()+"/command/runshortcut" {
			commandRunShortcut(string(msg.Payload()))
		}

	})

	token.Wait()
	if token.Error() != nil {
		log.Printf("Token error: %s\n", token.Error())
	}
}

func updateVolume(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/volume", 0, false, strconv.Itoa(getCurrentVolume()))
	token.Wait()
}

func updateMute(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/mute", 0, true, strconv.FormatBool(getMuteStatus()))
	token.Wait()
}

func updateBrightness(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/brightness", 0, false, strconv.Itoa(getCurrentBrightness()))
	token.Wait()
}

func getBatteryChargePercent() string {

	output := getCommandOutput("/usr/bin/pmset", "-g", "batt")

	// $ /usr/bin/pmset -g batt
	// Now drawing from 'Battery Power'
	//  -InternalBattery-0 (id=4653155)        100%; discharging; 20:00 remaining present: true

	r := regexp.MustCompile(`(\d+)%`)
	res := r.FindStringSubmatch(output)
	if len(res) == 0 {
		return ""
	}

	return res[1]
}

func updateBattery(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/battery", 0, false, getBatteryChargePercent())
	token.Wait()
}

func updateStatus(client mqtt.Client) {
	token := client.Publish(getTopicPrefix()+"/status/alive", 0, true, "true")
	token.Wait()
}

func main() {

	log.Println("Started")

	var c config
	c.getConfig()

	hostname = c.Hostname
	var wg sync.WaitGroup
	mqttClient := getMQTTClient(c.Ip, c.Port, c.User, c.Password)

	statusTicker := time.NewTicker(60 * time.Second)
	volumeTicker := time.NewTicker(60 * time.Second)
	batteryTicker := time.NewTicker(60 * time.Second)
	brightnessTicker := time.NewTicker(60 * time.Second)

	// Initial updates
	updateStatus(mqttClient)
	updateVolume(mqttClient)
	updateMute(mqttClient)
	updateBattery(mqttClient)
	updateBrightness(mqttClient)

	wg.Add(1)
	go func() {
		for {
			select {
			// Publish alive status every minute
			case _ = <-statusTicker.C:
				updateStatus(mqttClient)

			case _ = <-volumeTicker.C:
				updateVolume(mqttClient)
				updateMute(mqttClient)

			case _ = <-batteryTicker.C:
				updateBattery(mqttClient)
                
			case _ = <-brightnessTicker.C:
				updateBrightness(mqttClient)
			}
		}
	}()

	wg.Wait()

}