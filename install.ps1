$links = @()

if ($env:KAMIDERE_INSTALLER_URL) {
    $links += $env:KAMIDERE_INSTALLER_URL
}

$links += "https://github.com/Equicord/Equilotl/releases/latest/download/KamidereCli.exe"
$links += "https://github.com/Equicord/Equilotl/releases/latest/download/EquilotlCli.exe"

$outfile = "$env:TEMP\KamidereCli.exe"

Write-Output "Downloading Kamidere installer to $outfile"

$downloaded = $false
foreach ($link in $links) {
    try {
        Invoke-WebRequest -Uri $link -OutFile $outfile -ErrorAction Stop
        $downloaded = $true
        break
    }
    catch {
        continue
    }
}

if (-not $downloaded) {
    throw "Failed to download Kamidere installer."
}

Write-Output ""

Start-Process -Wait -NoNewWindow -FilePath $outfile

Remove-Item -Force $outfile
