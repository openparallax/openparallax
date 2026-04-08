//go:build windows

package platform

import "fmt"

func clipboardReadCmd() ([]string, error) {
	return []string{"powershell.exe", "-Command", "Get-Clipboard"}, nil
}

func clipboardWriteCmd() ([]string, error) {
	return []string{"powershell.exe", "-Command", "Set-Clipboard", "-Value"}, nil
}

func openCmd(target string) []string {
	return []string{"cmd", "/c", "start", "", target}
}

func notifyCmd(title, message string) []string {
	return []string{"powershell.exe", "-Command",
		fmt.Sprintf(`[System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms') | Out-Null; $n = New-Object System.Windows.Forms.NotifyIcon; $n.Icon = [System.Drawing.SystemIcons]::Information; $n.Visible = $true; $n.ShowBalloonTip(5000, '%s', '%s', 'Info')`, title, message)}
}

func screenshotCmd(outputPath string) []string {
	return []string{"powershell.exe", "-Command",
		fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $bmp = New-Object System.Drawing.Bitmap([System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width, [System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height); $g = [System.Drawing.Graphics]::FromImage($bmp); $g.CopyFromScreen(0,0,0,0,$bmp.Size); $bmp.Save('%s')`, outputPath)}
}
