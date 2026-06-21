package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

const (
	distroName      = "eXodite"
	distroNameLower = "exodite"
	distroID        = "exodite"
)

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[1;32m"
	yellow = "\033[1;33m"
	cyan   = "\033[1;36m"
	purple = "\033[1;35m"
	white  = "\033[1;37m"
)

const (
	gpuNvidia  = "NVIDIA (proprietary)"
	gpuOpenSrc = "Open Source (Intel / AMD / Nouveau)"
	gpuNone    = "None (No extra drivers)"
)

type Config struct {
	Disk          string
	DiskSizeBytes uint64
	PartLayout    string
	RootSizeGB    int
	GPU           string
	Desktop       string
	Hostname      string
	Username      string
	Password      string
	RootPass      string
	Timezone      string
	Keymap        string
	Locale        string
	EfiDevice     string
	RootDevice    string
	HomeDevice    string
}

func main() {
	if os.Getuid() != 0 {
		fmt.Println(red + "you need root privileges to run this installer" + reset)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println(red + "\n\n Install cancelled by user and Cleaning up the mounts" + reset)
		cleanupMounts()
		os.Exit(130)
	}()

	// Clear the screen for a fresh boot look
	fmt.Print("\033[H\033[2J")

	// Huge purple EXODITE banner
	fmt.Println(purple + `
███████╗██╗  ██╗██████╗ ██████╗ ██╗████████╗███████╗
██╔════╝╚██╗██╔╝██╔═══██╗██╔══██╗██║╚══██╔══╝██╔════╝
█████╗   ╚███╔╝ ██║   ██║██║  ██║██║   ██║   █████╗  
██╔══╝   ██╔██╗ ██║   ██║██║  ██║██║   ██║   ██╔══╝  
███████╗██╔╝ ██╗╚██████╔╝██████╔╝██║   ██║   ███████╗
╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝   ╚═╝   ╚══════╝
` + reset)

	fmt.Println(purple + "======")
	fmt.Printf("      Welcome to the %s Linux Installer\n", distroName)
	fmt.Println("======" + reset)

	var cfg Config
	runWizard(&cfg)

	fmt.Println(green + "\n[+] Config looks good installation starting" + reset)
	if err := runInstaller(&cfg); err != nil {
		fmt.Printf(red+"\nthe install failed: %v\n"+reset, err)
		cleanupMounts()
		os.Exit(1)
	}

	fmt.Println(green + "\n ")
	fmt.Printf("  %s is finally installed\n", distroName)
	fmt.Println("  u can reboot now")
	fmt.Println("" + reset)
}

func runWizard(cfg *Config) {
	reader := bufio.NewReader(os.Stdin)

	disks, err := listBlockDevices()
	if err != nil || len(disks) == 0 {
		fmt.Println(red + "no disks fond " + reset)
		os.Exit(1)
	}
	fmt.Println(yellow + "\nFound these ssd:" + reset)
	for i, d := range disks {
		fmt.Printf(" [%d] /dev/%s (%s)\n", i, d.name, formatBytes(d.size))
	}
	for {
		fmt.Print("Pick a drive: ")
		input, _ := reader.ReadString('\n')
		idx, err := strconv.Atoi(strings.TrimSpace(input))
		if err == nil && idx >= 0 && idx < len(disks) {
			cfg.Disk = "/dev/" + disks[idx].name
			cfg.DiskSizeBytes = disks[idx].size
			break
		}
		fmt.Println(red + "u cant choose that vro" + reset)
	}

	fmt.Println(yellow + "\nHow do you want to partition it?" + reset)
	fmt.Println(" [1] Standard (Single Btrfs partition with subvolumes (reccomended)")
	fmt.Println(" [2] Split Home (Separate Ext4 partitions for root and home)")
	fmt.Println(" [3] Dual Boot (Install alongside zereneOs ofc / peak distro btw )")
	for {
		fmt.Print("Choose an option [1-3]: ")
		input, _ := reader.ReadString('\n')
		switch strings.TrimSpace(input) {
		case "1":
			cfg.PartLayout = "standard"
		case "2":
			cfg.PartLayout = "split"
		case "3":
			cfg.PartLayout = "dualboot"
		default:
			fmt.Println(red + "brochacho, it's gotta be 1, 2, or 3" + reset)
			continue
		}
		break
	}

	if cfg.PartLayout == "dualboot" {
		maxGB := int(cfg.DiskSizeBytes / (1024 * 1024 * 1024))
		for {
			fmt.Printf("How many GBs do you want to give %s? (Max ~%d GB): ", distroName, maxGB)
			input, _ := reader.ReadString('\n')
			size, err := strconv.Atoi(strings.TrimSpace(input))
			if err == nil && size > 15 && size < maxGB {
				cfg.RootSizeGB = size
				break
			}
			fmt.Println(red + "Needs to be a valid number bigger than 15 GB" + reset)
		}
	}

	fmt.Println(yellow + "\nWhat GPU drivers" + reset)
	fmt.Printf(" [1] %s\n", gpuNvidia)
	fmt.Printf(" [2] %s\n", gpuOpenSrc)
	fmt.Printf(" [3] %s\n", gpuNone)
	for {
		fmt.Print("Pick an option [1-3] (and to xwcvt 4 is not an answer): ")
		input, _ := reader.ReadString('\n')
		switch strings.TrimSpace(input) {
		case "1":
			cfg.GPU = gpuNvidia
		case "2":
			cfg.GPU = gpuOpenSrc
		case "3":
			cfg.GPU = gpuNone
		default:
			fmt.Println(red + "i fucking said either choose 1, 2, or 3." + reset)
			continue
		}
		break
	}

	fmt.Println(yellow + "\nPick a desktop environment:" + reset)
	fmt.Println(" [1] KDE Plasma (Shiny and customizable)")
	fmt.Println(" [2] XFCE4 (Light and classic)")
	fmt.Println(" [3] GNOME (Clean and modern)")
	fmt.Println(" [4] Minimal (Just the terminal, no GUI)")
	fmt.Println(purple + " [5] Hyprland (Dynamic tiling Wayland compositor)" + reset)
	for {
		fmt.Print("Which one do you want? [1-5]: ")
		input, _ := reader.ReadString('\n')
		switch strings.TrimSpace(input) {
		case "1":
			cfg.Desktop = "KDE Plasma"
		case "2":
			cfg.Desktop = "XFCE4"
		case "3":
			cfg.Desktop = "GNOME"
		case "4":
			cfg.Desktop = "Minimal"
		case "5":
			cfg.Desktop = "Hyprland"
		default:
			fmt.Println(red + " pick between 1 and 5." + reset)
			continue
		}
		break
	}

	hnRegex := regexp.MustCompile(`^[a-zA-col1-9][a-zA-Z0-9\-]{1,62}$`)
	for {
		fmt.Print("\nGive your machine a hostname: ")
		hn, _ := reader.ReadString('\n')
		hn = strings.TrimSpace(hn)
		if hnRegex.MatchString(hn) {
			cfg.Hostname = hn
			break
		}
		fmt.Println(red + "js choose a normal name bro" + reset)
	}

	usrRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)
	for {
		fmt.Print("Set up a username (non-root ofc): ")
		usr, _ := reader.ReadString('\n')
		usr = strings.TrimSpace(usr)
		if usrRegex.MatchString(usr) && usr != "root" {
			cfg.Username = usr
			break
		}
		fmt.Println(red + "Username looks wrong. Stick to standard lowercase letters and numbers" + reset)
	}

	for {
		fmt.Print("Type a password for this user: ")
		p1, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Print("\nType it again to confirm: ")
		p2, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if len(p1) >= 6 && string(p1) == string(p2) {
			cfg.Password = string(p1)
			break
		}
		fmt.Println(red + "Passwords dont match" + reset)
	}

	for {
		fmt.Print("Type a password for the root account: ")
		p1, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Print("\nType it again to confirm root password: ")
		p2, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if len(p1) >= 6 && string(p1) == string(p2) {
			cfg.RootPass = string(p1)
			break
		}
		fmt.Println(red + "Passwords dont match" + reset)
	}

	cfg.Timezone = "UTC"
	cfg.Keymap = "us"
	cfg.Locale = "en_US.UTF-8"
}

func runInstaller(cfg *Config) error {
	type step struct {
		name string
		fn   func(*Config) error
	}
	steps := []step{
		{"Partitioning the disk", partitionDisk},
		{"Installing base system packages", installBase},
		{"doing more stuff", configure},
	}

	for _, s := range steps {
		fmt.Println(purple + "\n=== " + s.name + " ===" + reset)
		if err := s.fn(cfg); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

func partitionDisk(cfg *Config) error {
	if cfg.PartLayout == "dualboot" {
		return partitionDiskDualboot(cfg)
	}

	fmt.Printf("Setting up GPT partition table on %s...\n", cfg.Disk)
	if err := exec.Command("parted", "-s", cfg.Disk, "mklabel", "gpt").Run(); err != nil {
		return fmt.Errorf("couldn't create disk label: %v", err)
	}

	if cfg.PartLayout == "standard" {
		if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "ESP", "fat32", "1MiB", "513MiB").Run(); err != nil {
			return fmt.Errorf("couldn't make EFI partition: %v", err)
		}
		if err := exec.Command("parted", "-s", cfg.Disk, "set", "1", "esp", "on").Run(); err != nil {
			return fmt.Errorf("couldn't set ESP flag: %v", err)
		}
		if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "root", "btrfs", "513MiB", "100%").Run(); err != nil {
			return fmt.Errorf("couldn't make root partition: %v", err)
		}
	} else if cfg.PartLayout == "split" {
		if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "ESP", "fat32", "1MiB", "513MiB").Run(); err != nil {
			return fmt.Errorf("couldn't make split EFI partition: %v", err)
		}
		if err := exec.Command("parted", "-s", cfg.Disk, "set", "1", "esp", "on").Run(); err != nil {
			return fmt.Errorf("couldn't flag split ESP: %v", err)
		}
		if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "root", "ext4", "513MiB", "41.5GiB").Run(); err != nil {
			return fmt.Errorf("couldn't make split root partition: %v", err)
		}
		if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "home", "ext4", "41.5GiB", "100%").Run(); err != nil {
			return fmt.Errorf("couldn't make split home partition: %v", err)
		}
	}

	time.Sleep(2 * time.Second)
	newParts, err := getDiskPartitions(cfg.Disk)
	if err != nil || len(newParts) < 2 {
		return fmt.Errorf("kernel didnt catch the new partitions in time: %v", err)
	}

	var efi, root, home string
	for _, p := range newParts {
		if strings.HasSuffix(p, "1") || strings.HasSuffix(p, "p1") {
			efi = p
		} else if strings.HasSuffix(p, "2") || strings.HasSuffix(p, "p2") {
			root = p
		} else if strings.HasSuffix(p, "3") || strings.HasSuffix(p, "p3") {
			home = p
		}
	}

	if efi == "" || root == "" {
		return fmt.Errorf("lost track of partitions. EFI=%s Root=%s", efi, root)
	}

	cfg.EfiDevice = efi
	cfg.RootDevice = root
	cfg.HomeDevice = home

	fmt.Printf("Formatting the new partitions (EFI: %s, Root: %s)...\n", efi, root)
	if err := exec.Command("mkfs.vfat", "-F32", efi).Run(); err != nil {
		return fmt.Errorf("couldn't format EFI partition: %v", err)
	}

	if cfg.PartLayout == "standard" {
		if err := exec.Command("mkfs.btrfs", "-f", root).Run(); err != nil {
			return fmt.Errorf("couldn't format Btrfs root: %v", err)
		}
		if err := createBtrfsSubvolumes("/mnt_tmp", cfg); err != nil {
			return fmt.Errorf("couldn't create Btrfs subvolumes: %v", err)
		}
	} else {
		if err := exec.Command("mkfs.ext4", "-F", root).Run(); err != nil {
			return fmt.Errorf("couldn't format ext4 root: %v", err)
		}
		if cfg.PartLayout == "split" && home != "" {
			if err := exec.Command("mkfs.ext4", "-F", home).Run(); err != nil {
				return fmt.Errorf("couldnt format ext4 home: %v", err)
			}
		}
	}

	return mountTargetLayout(cfg)
}

func partitionDiskDualboot(cfg *Config) error {
	fmt.Println("Checking things out for dual boot setup")
	parts, err := getDiskPartitions(cfg.Disk)
	if err != nil || len(parts) == 0 {
		return fmt.Errorf("couldnt read existing partitions: %v", err)
	}

	var efiDevice string
	for _, p := range parts {
		out, _ := exec.Command("blkid", "-o", "value", "-s", "TYPE", p).Output()
		if strings.TrimSpace(string(out)) == "vfat" {
			efiDevice = p
			break
		}
	}
	if efiDevice == "" {
		return fmt.Errorf("couldn't find an existing EFI partition")
	}

	startOffset := "40000MiB"
	if out, err := exec.Command("parted", "-s", cfg.Disk, "print", "free").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(lines[i], "Free Space") {
				fields := strings.Fields(lines[i])
				if len(fields) > 0 {
					startOffset = fields[0]
					break
				}
			}
		}
	}

	fmt.Printf("Dropping the new root partition in the free space around %s \n", startOffset)
	if err := exec.Command("parted", "-s", cfg.Disk, "mkpart", "root", "ext4", startOffset, "100%").Run(); err != nil {
		return fmt.Errorf("couldnt partition the free space: %v", err)
	}

	time.Sleep(2 * time.Second)
	updatedParts, _ := getDiskPartitions(cfg.Disk)
	rootDevice := updatedParts[len(updatedParts)-1]

	cfg.EfiDevice = efiDevice
	cfg.RootDevice = rootDevice

	fmt.Printf("[*] Formatting partition %s as ext4...\n", rootDevice)
	if err := exec.Command("mkfs.ext4", "-F", rootDevice).Run(); err != nil {
		return fmt.Errorf("couldnt format the dual-boot root partition: %v", err)
	}

	return mountTargetLayout(cfg)
}

func createBtrfsSubvolumes(tmpMount string, cfg *Config) error {
	os.MkdirAll(tmpMount, 0755)
	if err := exec.Command("mount", cfg.RootDevice, tmpMount).Run(); err != nil {
		return fmt.Errorf("couldnt mount root partition for subvolume creation: %v", err)
	}
	defer exec.Command("umount", tmpMount).Run()

	subvols := []string{"@", "@/.snapshots", "@/var", "@/opt", "@/root", "@/tmp", "@/usr/local"}
	for _, sv := range subvols {
		targetPath := filepath.Join(tmpMount, sv)
		
		parentDir := filepath.Dir(targetPath)
		if parentDir != tmpMount {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("couldn't make parent folder for subvolume %s: %v", sv, err)
			}
		}

		if err := exec.Command("btrfs", "subvolume", "create", targetPath).Run(); err != nil {
			return fmt.Errorf("couldnt create subvolume %s: %v", sv, err)
		}
	}
	return nil
}

func mountTargetLayout(cfg *Config) error {
	fmt.Println("Mounting all the folders to prepare for install")
	os.MkdirAll("/mnt", 0755)

	if cfg.PartLayout == "standard" {
		if err := exec.Command("mount", "-o", "subvol=@", cfg.RootDevice, "/mnt").Run(); err != nil {
			return fmt.Errorf("couldn't mount root subvolume: %v", err)
		}
		
		mappings := []struct{ subvol, target string }{
			{"@/.snapshots", "/mnt/.snapshots"},
			{"@/var", "/mnt/var"},
			{"@/opt", "/mnt/opt"},
			{"@/root", "/mnt/root"},
			{"@/tmp", "/mnt/tmp"},
			{"@/usr/local", "/mnt/usr/local"},
		}
		for _, m := range mappings {
			os.MkdirAll(m.target, 0755)
			if err := exec.Command("mount", "-o", "subvol="+m.subvol, cfg.RootDevice, m.target).Run(); err != nil {
				return fmt.Errorf("couldn't mount subvolume %s: %v", m.subvol, err)
			}
		}
	} else {
		if err := exec.Command("mount", cfg.RootDevice, "/mnt").Run(); err != nil {
			return fmt.Errorf("couldn't mount root partition: %v", err)
		}
		if cfg.PartLayout == "split" && cfg.HomeDevice != "" {
			os.MkdirAll("/mnt/home", 0755)
			if err := exec.Command("mount", cfg.HomeDevice, "/mnt/home").Run(); err != nil {
				return fmt.Errorf("couldnt mount home partition: %v", err)
			}
		}
	}

	os.MkdirAll("/mnt/boot/efi", 0755)
	if err := exec.Command("mount", cfg.EfiDevice, "/mnt/boot/efi").Run(); err != nil {
		return fmt.Errorf("couldn't mount EFI partition: %v", err)
	}

	return nil
}

func installBase(cfg *Config) error {
	fmt.Println("Refreshing repos")
	
	zypperArgs := []string{
		"--installroot=/mnt",
		"--non-interactive",
		"install",
		"--no-recommends",
		"-t", "pattern", "enhanced_base",
	}
	cmd := exec.Command("zypper", zypperArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("couldnt install base packages: %v", err)
	}

	extraPackages := []string{"kernel-default", "grub2", "grub2-x86_64-efi", "NetworkManager", "sudo"}
	switch cfg.GPU {
	case gpuNvidia:
		extraPackages = append(extraPackages, "xf86-video-nouveau")
	case gpuOpenSrc:
		extraPackages = append(extraPackages, "xf86-video-intel", "xf86-video-amdgpu", "mesa-dri")
	}

	switch cfg.Desktop {
	case "KDE Plasma":
		extraPackages = append(extraPackages, "patterns-kde-kde", "sddm")
	case "XFCE4":
		extraPackages = append(extraPackages, "patterns-xfce-xfce", "lightdm")
	case "GNOME":
		extraPackages = append(extraPackages, "patterns-gnome-gnome", "gdm")
	case "Hyprland":
		extraPackages = append(extraPackages, "hyprland", "sddm", "waybar", "kitty")
	}

	fmt.Println(" Installing extra packages and desktop setup")
	installArgs := append([]string{"--installroot=/mnt", "--non-interactive", "install"}, extraPackages...)
	cmd = exec.Command("zypper", installArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func configure(cfg *Config) error {
	fmt.Println("Writing configs and wrapping things up")

	if err := writeFstab(cfg); err != nil {
		return fmt.Errorf("couldn't generate fstab: %v", err)
	}

	if err := os.WriteFile("/mnt/etc/hostname", []byte(cfg.Hostname+"\n"), 0644); err != nil {
		return fmt.Errorf("couldn't write hostname file: %v", err)
	}

	script := "#!/bin/bash\n"
	script += fmt.Sprintf("ln -sf /usr/share/zoneinfo/%s /etc/localtime\n", cfg.Timezone)
	script += fmt.Sprintf("echo 'KEYMAP=%s' > /etc/vconsole.conf\n", cfg.Keymap)
	script += fmt.Sprintf("echo 'LANG=%s' > /etc/locale.conf\n", cfg.Locale)
	script += fmt.Sprintf("cat > /etc/os-release << 'EOF'\nNAME=\"%s Linux\"\nID=%s\nPRETTY_NAME=\"%s Linux\"\nEOF\n", distroName, distroID, distroName)
	script += fmt.Sprintf("useradd -m -G wheel,users -s /bin/bash '%s'\n", cfg.Username)

	script += "chpasswd << 'EOF'\n"
	script += fmt.Sprintf("root:%s\n", cfg.RootPass)
	script += fmt.Sprintf("%s:%s\n", cfg.Username, cfg.Password)
	script += "EOF\n"

	script += `echo '%wheel ALL=(ALL:ALL) ALL' > /etc/sudoers.d/10-wheel
chmod 440 /etc/sudoers.d/10-wheel
`
	script += "mkinitrd\n"
	script += fmt.Sprintf("grub2-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=%s\n", distroName)
	if cfg.PartLayout == "dualboot" {
		script += "echo 'GRUB_DISABLE_OS_PROBER=false' >> /etc/default/grub\n"
	}
	script += "grub2-mkconfig -o /boot/grub2/grub.cfg\n"
	script += "systemctl enable NetworkManager\n"

	switch cfg.Desktop {
	case "KDE Plasma":
		script += "systemctl enable sddm\n"
	case "XFCE4":
		script += "systemctl enable lightdm\n"
	case "GNOME":
		script += "systemctl enable gdm\n"
	case "Hyprland":
		script += "systemctl enable sddm\n"
	}

	scriptPath := "/mnt/setup.sh"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("couldn't write the setup script: %v", err)
	}
	defer os.Remove(scriptPath)

	fmt.Println("Chrooting into the new system")
	
	apiDirs := []string{"/dev", "/proc", "/sys", "/run"}
	for _, d := range apiDirs {
		if err := exec.Command("mount", "--bind", d, "/mnt"+d).Run(); err != nil {
			return fmt.Errorf("couldn't bind mount %s: %v", d, err)
		}
	}
	defer func() {
		for i := len(apiDirs) - 1; i >= 0; i-- {
			exec.Command("umount", "-l", "/mnt"+apiDirs[i]).Run()
		}
	}()

	chrootCmd := exec.Command("chroot", "/mnt", "/setup.sh")
	chrootCmd.Stdout = os.Stdout
	chrootCmd.Stderr = os.Stderr
	return chrootCmd.Run()
}

func writeFstab(cfg *Config) error {
	rootUUID, err := getUUID(cfg.RootDevice)
	if err != nil || rootUUID == "" {
		return fmt.Errorf("couldn't get UUID for root partition: %v", err)
	}
	efiUUID, err := getUUID(cfg.EfiDevice)
	if err != nil || efiUUID == "" {
		return fmt.Errorf("couldn't get UUID for EFI partition: %v", err)
	}

	var lines []string
	if cfg.PartLayout == "standard" {
		subvols := []string{"@", "@/.snapshots", "@/var", "@/opt", "@/root", "@/tmp", "@/usr/local"}
		for _, sv := range subvols {
			mountPoint := strings.TrimPrefix(sv, "@")
			if mountPoint == "" {
				mountPoint = "/"
			}
			opts := "defaults,noatime,subvol=" + sv
			lines = append(lines, fmt.Sprintf("UUID=%s %s btrfs %s 0 0", rootUUID, mountPoint, opts))
		}
	} else {
		lines = append(lines, fmt.Sprintf("UUID=%s / ext4 defaults,noatime 0 1", rootUUID))
		if cfg.PartLayout == "split" && cfg.HomeDevice != "" {
			homeUUID, err := getUUID(cfg.HomeDevice)
			if err != nil || homeUUID == "" {
				return fmt.Errorf("couldn't get UUID for home partition: %v", err)
			}
			lines = append(lines, fmt.Sprintf("UUID=%s /home ext4 defaults,noatime 0 2", homeUUID))
		}
	}

	lines = append(lines, fmt.Sprintf("UUID=%s /boot/efi vfat defaults,fmask=0077,dmask=0077 0 2", efiUUID))
	return os.WriteFile("/mnt/etc/fstab", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func getUUID(device string) (string, error) {
	out, err := exec.Command("lsblk", "-d", "-no", "UUID", device).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type blockDevice struct {
	name string
	size uint64
}

func listBlockDevices() ([]blockDevice, error) {
	out, err := exec.Command("lsblk", "-d", "-n", "-o", "NAME,SIZE,TYPE").Output()
	if err != nil {
		return nil, err
	}

	var devices []blockDevice
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 3 && f[2] == "disk" && !strings.HasPrefix(f[0], "loop") && !strings.HasPrefix(f[0], "airoot") {
			size, _ := getDeviceSize(f[0])
			devices = append(devices, blockDevice{name: f[0], size: size})
		}
	}
	return devices, nil
}

func getDeviceSize(name string) (uint64, error) {
	out, err := os.ReadFile(filepath.Join("/sys/class/block", name, "size"))
	if err != nil {
		return 0, err
	}
	sectors, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, err
	}
	return sectors * 512, nil
}

func getDiskPartitions(disk string) ([]string, error) {
	baseName := filepath.Base(disk)
	var parts []string
	files, err := filepath.Glob("/dev/" + baseName + "*")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f != disk {
			parts = append(parts, f)
		}
	}
	return parts, nil
}

func cleanupMounts() {
	exec.Command("umount", "-R", "/mnt").Run()
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
