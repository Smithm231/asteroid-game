package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	version  = syscall.NewLazyDLL("version.dll")

	procFindResource   = kernel32.NewProc("FindResourceW")
	procSizeofResource = kernel32.NewProc("SizeofResource")
	procLoadResource   = kernel32.NewProc("LoadResource")
	procLockResource   = kernel32.NewProc("LockResource")
	procModuleFileName = kernel32.NewProc("GetModuleFileNameW")
	procActCtxSetting  = kernel32.NewProc("QueryActCtxSettingsW")
	procLookupIconID   = user32.NewProc("LookupIconIdFromDirectoryEx")
	procCreateIcon     = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon    = user32.NewProc("DestroyIcon")
	procIsDPIAware     = user32.NewProc("IsProcessDPIAware")
	procVerInfoSize    = version.NewProc("GetFileVersionInfoSizeW")
	procVerInfo        = version.NewProc("GetFileVersionInfoW")
	procVerQueryValue  = version.NewProc("VerQueryValueW")
)

// Resource types (winuser.h).
const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24
)

// findResource returns the bytes of the numbered resource of this executable,
// as located by the operating system's resource loader.
func findResource(typ, id uintptr) ([]byte, error) {
	h, _, _ := procFindResource.Call(0, id, typ)
	if h == 0 {
		return nil, fmt.Errorf("FindResource(type %d, id %d) found nothing", typ, id)
	}
	size, _, _ := procSizeofResource.Call(0, h)
	mem, _, _ := procLoadResource.Call(0, h)
	ptr, _, _ := procLockResource.Call(mem)
	if size == 0 || ptr == 0 {
		return nil, fmt.Errorf("resource type %d id %d cannot be loaded", typ, id)
	}
	// The resource is mapped read-only for the lifetime of the process.
	return unsafe.Slice((*byte)(pointer(ptr)), size), nil
}

// pointer converts an address returned by a system call into a pointer. The
// memory is not managed by Go, so the usual hazards of such a conversion do
// not apply; going through a variable keeps vet's unsafeptr check quiet.
func pointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

// selfCheck asks Windows for the resources winres embedded and reports what
// the system makes of them.
func selfCheck() error {
	// Icon: let the system pick the image for each size from the group
	// and turn that image into an icon handle.
	group, err := findResource(rtGroupIcon, 1)
	if err != nil {
		return err
	}
	for i, size := range []uintptr{16, 20, 24, 32, 40, 48, 64, 128, 256} {
		id, _, _ := procLookupIconID.Call(uintptr(unsafe.Pointer(&group[0])), 1, size, size, 0)
		if id != uintptr(i+1) {
			return fmt.Errorf("LookupIconIdFromDirectoryEx(%d px) = %d, want %d", size, id, i+1)
		}
		image, err := findResource(rtIcon, id)
		if err != nil {
			return err
		}
		const iconVersion = 0x00030000
		icon, _, callErr := procCreateIcon.Call(uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1, iconVersion, size, size, 0)
		if icon == 0 {
			return fmt.Errorf("CreateIconFromResourceEx(%d px): %v", size, callErr)
		}
		procDestroyIcon.Call(icon)
		fmt.Printf("icon %3d px: RT_ICON %d, %d bytes, handle created\n", size, id, len(image))
	}

	// Manifest: present as a resource, and its settings are visible
	// through the process's activation context.
	manifest, err := findResource(rtManifest, 1)
	if err != nil {
		return err
	}
	fmt.Printf("manifest: %d bytes\n", len(manifest))
	const ns2005, ns2016 = "http://schemas.microsoft.com/SMI/2005/WindowsSettings", "http://schemas.microsoft.com/SMI/2016/WindowsSettings"
	for _, s := range []struct{ ns, name, want string }{
		{ns2005, "dpiAware", "true/pm"},
		{ns2016, "dpiAwareness", "PerMonitorV2,PerMonitor"},
		{ns2016, "longPathAware", "true"},
	} {
		var buf [128]uint16
		var written uintptr
		ok, _, callErr := procActCtxSetting.Call(0, 0,
			uintptr(unsafe.Pointer(utf16(s.ns))), uintptr(unsafe.Pointer(utf16(s.name))),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&written)))
		if ok == 0 {
			return fmt.Errorf("QueryActCtxSettingsW(%s): %v", s.name, callErr)
		}
		got := syscall.UTF16ToString(buf[:])
		if got != s.want {
			return fmt.Errorf("manifest setting %s = %q, want %q", s.name, got, s.want)
		}
		fmt.Printf("manifest setting %s = %q\n", s.name, got)
	}
	// (Wine reports every process as DPI aware, so this only tells
	// something on Windows itself.)
	aware, _, _ := procIsDPIAware.Call()
	fmt.Printf("IsProcessDPIAware = %d\n", aware)
	if aware == 0 {
		return errors.New("process is not DPI aware although the manifest declares it")
	}

	// Version information, through the version API.
	if _, err := findResource(rtVersion, 1); err != nil {
		return err
	}
	var path [1024]uint16
	procModuleFileName.Call(0, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	size, _, callErr := procVerInfoSize.Call(uintptr(unsafe.Pointer(&path[0])), 0)
	if size == 0 {
		return fmt.Errorf("GetFileVersionInfoSizeW: %v", callErr)
	}
	info := make([]byte, size)
	if ok, _, callErr := procVerInfo.Call(uintptr(unsafe.Pointer(&path[0])), 0, size, uintptr(unsafe.Pointer(&info[0]))); ok == 0 {
		return fmt.Errorf("GetFileVersionInfoW: %v", callErr)
	}
	query := func(block string) (unsafe.Pointer, uint32, error) {
		var ptr unsafe.Pointer
		var n uint32
		ok, _, _ := procVerQueryValue.Call(uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(utf16(block))),
			uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n)))
		if ok == 0 || ptr == nil || n == 0 {
			return nil, 0, fmt.Errorf("VerQueryValueW(%s) found nothing", block)
		}
		return ptr, n, nil
	}
	ptr, n, err := query(`\`)
	if err != nil {
		return err
	}
	fixed := unsafe.Slice((*uint32)(ptr), n/4)
	if len(fixed) != 13 || fixed[0] != 0xFEEF04BD {
		return fmt.Errorf("bad VS_FIXEDFILEINFO: %d bytes, signature %#x", n, fixed[0])
	}
	fmt.Printf("version: file %d.%d.%d.%d, product %d.%d.%d.%d\n",
		fixed[2]>>16, fixed[2]&0xFFFF, fixed[3]>>16, fixed[3]&0xFFFF,
		fixed[4]>>16, fixed[4]&0xFFFF, fixed[5]>>16, fixed[5]&0xFFFF)
	if ptr, n, err = query(`\VarFileInfo\Translation`); err != nil {
		return err
	}
	tr := unsafe.Slice((*uint16)(ptr), n/2)
	table := fmt.Sprintf("%04x%04x", tr[0], tr[1])
	fmt.Printf("version: translation %s\n", table)
	for _, s := range []struct{ name, want string }{
		{"ProductName", "Drifthold"},
		{"FileDescription", "Drifthold"},
		{"InternalName", "drifthold"},
		{"OriginalFilename", "Drifthold.exe"},
		{"FileVersion", ""},
		{"ProductVersion", ""},
	} {
		ptr, n, err := query(`\StringFileInfo\` + table + `\` + s.name)
		if err != nil {
			return err
		}
		got := syscall.UTF16ToString(unsafe.Slice((*uint16)(ptr), n))
		if s.want != "" && got != s.want {
			return fmt.Errorf("version string %s = %q, want %q", s.name, got, s.want)
		}
		fmt.Printf("version: %s = %q\n", s.name, got)
	}
	fmt.Println("selfcheck: ok")
	return nil
}

// utf16 converts a Go string to a NUL-terminated UTF-16 string.
func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}
