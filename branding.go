package main

import (
	"fmt"
	"os"
	"runtime"

	"kamidereinstaller/buildinfo"
)

const (
	BrandName            = "Kamidere"
	InstallerDisplayName = "Kamidere Installer"
	InstallerGuiBinary   = "KamidereInstaller"
	InstallerCliBinary   = "KamidereCli"
	InstallerMacBundle   = "KamidereInstaller.app"
	InstallerMacArchive  = "KamidereInstaller.MacOS.zip"
	InstallerLinuxGui    = "KamidereInstaller-x11"
	InstallerLinuxCli    = "KamidereCli-linux"
	ClientAsarFile       = "kamidere.asar"
	LegacyClientAsarFile = "equicord.asar"
	ClientDataDir        = "KamidereData"
	LegacyClientDataDir  = "EquicordData"

	EnvUserDataDir       = "KAMIDERE_USER_DATA_DIR"
	LegacyEnvUserDataDir = "EQUICORD_USER_DATA_DIR"
	EnvInstallFile       = "KAMIDERE_DIRECTORY"
	LegacyEnvInstallFile = "EQUICORD_DIRECTORY"
	EnvDevInstall        = "KAMIDERE_DEV_INSTALL"
	LegacyEnvDevInstall  = "EQUICORD_DEV_INSTALL"
)

var (
	ClientRepositoryURL         = envOr("KAMIDERE_REPOSITORY_URL", "https://github.com/Equicord/Equicord")
	InstallerRepositoryURL      = envOr("KAMIDERE_INSTALLER_REPOSITORY_URL", "https://github.com/Equicord/Equilotl")
	ReleaseUrl                  = envOr("KAMIDERE_RELEASE_URL", "https://api.github.com/repos/Equicord/Equicord/releases/latest")
	ReleaseUrlFallback          = envOr("KAMIDERE_RELEASE_URL_FALLBACK", "https://equicord.org/releases/equicord")
	InstallerReleaseUrl         = envOr("KAMIDERE_INSTALLER_RELEASE_URL", "https://api.github.com/repos/Equicord/Equilotl/releases/latest")
	InstallerReleaseUrlFallback = envOr("KAMIDERE_INSTALLER_RELEASE_URL_FALLBACK", "https://equicord.org/releases/equilotl")
	InstallerDownloadBaseURL    = envOr("KAMIDERE_INSTALLER_DOWNLOAD_BASE_URL", "https://github.com/Equicord/Equilotl/releases/latest/download/")
	UserAgent                   = fmt.Sprintf("KamidereInstaller/%s (%s)", buildinfo.InstallerGitHash, InstallerRepositoryURL)
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvAny(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func InstallerAssetCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		if buildinfo.UiType == buildinfo.UiTypeCli {
			return []string{InstallerCliBinary + ".exe", "EquilotlCli.exe"}
		}
		return []string{InstallerGuiBinary + ".exe", "Equilotl.exe"}
	case "darwin":
		return []string{InstallerMacArchive, "Equilotl.MacOS.zip"}
	case "linux":
		if buildinfo.UiType == buildinfo.UiTypeCli {
			return []string{InstallerLinuxCli, "EquilotlCli-linux", "EquilotlCli-Linux"}
		}
		return []string{InstallerLinuxGui, "Equilotl-x11"}
	default:
		return nil
	}
}

func InstallerUpdateTempPrefix() string {
	return InstallerGuiBinary + "Update"
}
