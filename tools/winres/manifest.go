package main

import "fmt"

// supportedOSWindows10 is the compatibility GUID shared by Windows 10 and
// Windows 11. Declaring it makes Windows report its true version to the
// process and enables current behaviour instead of compatibility shims.
const supportedOSWindows10 = "{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"

// XML namespaces of the manifest's setting elements.
const (
	nsSettings2005 = "http://schemas.microsoft.com/SMI/2005/WindowsSettings"
	nsSettings2016 = "http://schemas.microsoft.com/SMI/2016/WindowsSettings"
)

// manifestXML returns the application manifest embedded as RT_MANIFEST.
//
// It declares that the game
//   - runs with the invoking user's privileges (no UAC elevation, and no
//     file system or registry virtualisation),
//   - targets Windows 10 and 11,
//   - is per-monitor DPI aware: PerMonitorV2 where available (Windows 10
//     1703 and later), falling back to per-monitor awareness, with the
//     older dpiAware element for systems that predate dpiAwareness,
//   - can use paths longer than MAX_PATH where the system allows it.
//
// No common-controls dependency is declared: the game draws its own
// interface and does not use Win32 controls.
func manifestXML(version [4]uint16) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="%s" version="%s" processorArchitecture="amd64"/>
  <description>%s</description>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="%s"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="%s">true/pm</dpiAware>
      <dpiAwareness xmlns="%s">PerMonitorV2,PerMonitor</dpiAwareness>
      <longPathAware xmlns="%s">true</longPathAware>
    </windowsSettings>
  </application>
</assembly>
`, productName, dotted(version), productName, supportedOSWindows10, nsSettings2005, nsSettings2016, nsSettings2016))
}
