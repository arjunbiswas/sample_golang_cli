package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
	"os/exec"
	"strings"
)

var repoNames = []string{"ionetcontainers/io-worker-vc", "ionetcontainers/io-worker-monitor", "ionetcontainers/io-launch"}
var validArchChoices = []string{"x86_64", "arm64", "aarch64"}
var validOSChoices = []string{"macOS", "Linux"}

// Returns true if string is contained
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Returns true if valid UUID is passed
func isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func getMacInfo() bool {
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	err := cmd.Run()
	return err == nil
}

func getPlatformArchitecture() string {
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("Unable to determine platform architecture")
	}
	return strings.TrimSpace(string(output))
}

func checkDocker() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	if err != nil {
		fmt.Println("Docker daemon is not running. Please start Docker and try again.")
		return false
	}
	return true
}

func getDockerImageIDsSorted(repoName string) []string {
	cmd := fmt.Sprintf("docker image ls --no-trunc --format '{{.Repository}}:{{.Tag}} {{.CreatedAt}} {{.ID}}' | grep -i '%s' | sort -r", repoName)
	output, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}
	sortedImages := strings.Split(strings.TrimSpace(string(output)), "\n")
	var imageIDs []string
	for _, img := range sortedImages {
		fields := strings.Fields(img)
		if len(fields) > 0 {
			imageIDs = append(imageIDs, fields[len(fields)-1])
		}
	}
	return imageIDs
}

func removeDockerImage(imageID string) {
	cmd := exec.Command("docker", "rmi", imageID)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func stopRunningContainers() {
	cmd := exec.Command("docker", "ps", "-q")
	output, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	containers := strings.TrimSpace(string(output))
	if containers != "" {
		fmt.Println("Stopping all running Docker containers...")
		args := append([]string{"docker", "stop"}, strings.Fields(containers)...)
		cmd := exec.Command(args[0], args[1:]...)
		err := cmd.Run()
		if err != nil {
			fmt.Println("Stopping containers failed, attempting to forcefully kill containers...")
			args := append([]string{"docker", "kill"}, strings.Fields(containers)...)
			cmd := exec.Command(args[0], args[1:]...)
			err := cmd.Run()
			if err != nil {
				fmt.Println("Failed to kill running Docker containers.")
				os.Exit(1)
			}
		}
	}
}

func checkGPUAvailability() bool {
	cmd := exec.Command("nvidia-smi")
	err := cmd.Run()
	if err != nil {
		fmt.Println("nvidia-smi failed - please rerun io-setup or contact support on discord")
		return false
	}
	return true
}

func checkNvidiaCTK() bool {
	cmd := exec.Command("nvidia-ctk", "--version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("NVIDIA Container Toolkit check failed - please rerun io-setup or contact support on discord")
		return false
	}
	if strings.Contains(string(output), "NVIDIA Container Toolkit CLI version") {
		return true
	} else {
		fmt.Println("NVIDIA Container Toolkit not installed - please rerun io-setup or contact support on discord")
		return false
	}
}

func constructDockerCommand(args *Arguments, architecture string) []string {
	dockerCommand := []string{
		"docker", "run", "-d",
		"-v", "/var/run/docker.sock:/var/run/docker.sock", "--network", "host",
	}
	dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("ARCH=%s", architecture))
	if architecture != "x86_64" {
		dockerCommand = append(dockerCommand, "--platform", "linux/amd64")
	}
	if args.DeviceName != "" {
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("DEVICE_NAME=%s", args.DeviceName))
	}
	if args.DeviceID != "" {
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("DEVICE_ID=%s", args.DeviceID))
	}
	if args.UserID != "" {
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("USER_ID=%s", args.UserID))
	}
	if args.OperatingSystem != "" {
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("OPERATING_SYSTEM=%s", args.OperatingSystem))
	}
	if args.UseGPUs != "" {
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("USEGPUS=%s", args.UseGPUs))
	}
	if args.OperatingSystem == "macOS" {
		macInfo, err := exec.Command("sh", "-c", "sysctl -a | grep machdep | awk -F': ' '{print \"\\\"\" $1 \"\\\": \\\"\" $2 \"\\\"\"}' | paste -sd, - | awk '{print \"{\" $0 \"}\" }'").Output()
		if err != nil {
			log.Fatal(err)
		}
		dockerCommand = append(dockerCommand, "-e", fmt.Sprintf("MAC_INFO=%s", strings.TrimSpace(string(macInfo))))
		dockerCommand = append(dockerCommand, "--pull", "always")
	}
	if args.Beta {
		dockerCommand = append(dockerCommand, "-e", "CURRENT_LOG_LEVEL=DEBUG", "-e", "ENVIRONMENT=DEV")
		dockerCommand = append(dockerCommand, "ionetcontainers/io-launch-beta:v0.1")
	} else {
		dockerCommand = append(dockerCommand, "ionetcontainers/io-launch:v0.1")
	}
	return dockerCommand
}

func loadCache() map[string]interface{} {
	cache := make(map[string]interface{})
	file, err := os.Open("ionet_device_cache.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cache)
	if err != nil {
		log.Fatal(err)
	}
	return cache
}

func saveCache(data map[string]interface{}) {
	file, err := os.Create("ionet_device_cache.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)
	encoder := json.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		log.Fatal(err)
	}
}

type Arguments struct {
	DeviceName      string
	DeviceID        string
	UserID          string
	OperatingSystem string
	UseGPUs         string
	Architecture    string
	Beta            bool
}

func main() {
	deviceName := flag.String("device_name", "", "")
	deviceID := flag.String("device_id", "", "")
	userID := flag.String("user_id", "", "")
	operatingSystem := flag.String("operating_system", "", "")
	useGPUs := flag.String("usegpus", "", "")
	architecture := flag.String("arch", "", "")
	beta := flag.Bool("beta", false, "")
	flag.Parse()

	cache := loadCache()

	args := new(Arguments)

	if *deviceName == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["device_name"])
	}
	if *deviceID == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["device_id"])
	}
	if *userID == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["user_id"])
	}
	if *operatingSystem == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["operating_system"])
	}
	if *useGPUs == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["usegpus"])
	}
	if *architecture == "" {
		args.DeviceName = fmt.Sprintf("%v", cache["arch"])
	}
	if *beta {
		args.DeviceName = fmt.Sprintf("%v", cache["beta"])
	}

	if !checkDocker() {
		os.Exit(1)
	}

	stopRunningContainers()

	for _, repoName := range repoNames {
		imageIDs := getDockerImageIDsSorted(repoName)
		if len(imageIDs) > 1 {
			fmt.Printf("removing stale images: %s\n", repoName)
			for _, imageID := range imageIDs[1:] {
				removeDockerImage(imageID)
			}
		}
	}

	for args.DeviceName == "" {
		fmt.Print("Enter device name: ")
		_, err := fmt.Scanln(&args.DeviceName)
		if err != nil {
			return
		}
		if args.DeviceName == "" {
			fmt.Println("Device name cannot be empty. Please enter a valid name.")
		}
	}

	for args.DeviceID == "" || !isValidUUID(args.DeviceID) {
		fmt.Print("Enter device ID (UUID): ")
		_, err := fmt.Scanln(&args.DeviceID)
		if err != nil {
			return
		}
		if !isValidUUID(args.DeviceID) {
			fmt.Println("Invalid UUID. Please enter a proper UUID as shown on the website dashboard.")
		}
	}

	for args.UserID == "" || !isValidUUID(args.UserID) {
		fmt.Print("Enter user ID (UUID): ")
		_, err := fmt.Scanln(&args.UserID)
		if err != nil {
			return
		}
		if !isValidUUID(args.UserID) {
			fmt.Println("Invalid UUID. Please enter a proper UUID as shown on the website dashboard.")
		}
	}

	for args.OperatingSystem == "" || !contains(validOSChoices, args.OperatingSystem) {
		fmt.Print("Enter operating system (macOS/Linux): ")
		_, err := fmt.Scanln(&args.OperatingSystem)
		if err != nil {
			return
		}
		if !contains(validOSChoices, args.OperatingSystem) {
			fmt.Printf("Invalid operating system. Please choose from %s.\n", strings.Join(validOSChoices, "/"))
		}
	}

	if args.OperatingSystem == "Windows" {
		args.Architecture = "x86_64"
	} else if args.Architecture == "" {
		args.Architecture = getPlatformArchitecture()
	}

	if args.OperatingSystem == "macOS" {
		args.UseGPUs = "false"
		fmt.Println("NOTE: If you see a warning regarding the platform mismatch (linux/amd64 vs. linux/arm64/v8), please ignore it. This is expected when running on macOS with M1/M2/M3 chips.")
	} else if args.UseGPUs == "" {
		for args.UseGPUs == "" || (args.UseGPUs != "true" && args.UseGPUs != "false") {
			fmt.Print("Does this system have an NVIDIA GPU which you want to use? (true/false): ")
			_, err := fmt.Scanln(&args.UseGPUs)
			if err != nil {
				return
			}
			if args.UseGPUs != "true" && args.UseGPUs != "false" {
				fmt.Println("Invalid input. Please enter 'true' or 'false'.")
			}
		}
	}

	if args.OperatingSystem != "macOS" && args.UseGPUs == "true" {
		if !checkGPUAvailability() || !checkNvidiaCTK() {
			os.Exit(1)
		}
	}

	if !checkDocker() {
		os.Exit(1)
	}

	if !contains(validOSChoices, args.OperatingSystem) {
		fmt.Printf("Error: Invalid operating system choice '%s'\n", args.OperatingSystem)
		os.Exit(1)
	}

	if !contains(validArchChoices, args.Architecture) {
		fmt.Printf("Platform %s - %s is not supported\n", args.Architecture, args.OperatingSystem)
		os.Exit(1)
	}

	if args.OperatingSystem == "macOS" && !getMacInfo() {
		fmt.Println("Your hardware isn’t Mac silicon (M1, M2, M3) chips, please select proper OS and chip from website")
		os.Exit(1)
	}

	cacheData := map[string]interface{}{
		"device_name":      args.DeviceName,
		"device_id":        args.DeviceID,
		"user_id":          args.UserID,
		"operating_system": args.OperatingSystem,
		"usegpus":          args.UseGPUs,
	}
	saveCache(cacheData)

	dockerCommand := constructDockerCommand(args, args.Architecture)

	cmd := exec.Command(dockerCommand[0], dockerCommand[1:]...)
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}
