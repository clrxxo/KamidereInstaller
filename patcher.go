/*
 * SPDX-License-Identifier: GPL-3.0
 * Kamidere Installer, a cross platform gui/cli app for installing Kamidere
 * Copyright (c) 2026 Kamidere contributors
 */

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	path "path/filepath"
	"strings"

	"kamidereinstaller/buildinfo"

	"github.com/ProtonMail/go-appdir"
)

var BaseDir string
var BaseDirErr error
var KamidereDirectory string
var ErrOriginalBackupMissing = errors.New("original app.asar backup is missing")

func pathExists(target string) bool {
	_, err := os.Stat(target)
	return err == nil
}

func pathIsDir(target string) bool {
	info, err := os.Stat(target)
	return err == nil && info.IsDir()
}

func isLocalKamidereBuildDir(target string) bool {
	if !pathIsDir(target) {
		return false
	}

	return pathExists(path.Join(target, "package.json")) &&
		pathExists(path.Join(target, "patcher.js"))
}

func findLocalDevelopmentBuild() string {
	if buildinfo.InstallerTag != buildinfo.VersionUnknown {
		return ""
	}

	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	exeDir := path.Dir(exePath)
	candidates := []string{
		path.Join(exeDir, "..", "Kamidere", "dist", "desktop"),
		path.Join(exeDir, "..", "KamidereCord", "dist", "desktop"),
		path.Join(exeDir, "dist", "desktop"),
	}

	for _, candidate := range candidates {
		candidate = path.Clean(candidate)
		if isLocalKamidereBuildDir(candidate) {
			return candidate
		}
	}

	return ""
}

func validateKamiderePayload() error {
	info, err := os.Stat(KamidereDirectory)
	if err != nil {
		if buildinfo.InstallerTag == buildinfo.VersionUnknown {
			return fmt.Errorf(
				"could not find a local %s build at %s. Run `pnpm build` in the sibling Kamidere project or use `pnpm inject` from that project",
				BrandName,
				KamidereDirectory,
			)
		}

		return fmt.Errorf("could not find %s payload at %s: %w", BrandName, KamidereDirectory, err)
	}

	if !info.IsDir() {
		return nil
	}

	requiredFiles := []string{
		path.Join(KamidereDirectory, "package.json"),
		path.Join(KamidereDirectory, "patcher.js"),
	}
	for _, requiredFile := range requiredFiles {
		if !pathExists(requiredFile) {
			return fmt.Errorf("local %s build is incomplete: missing %s", BrandName, requiredFile)
		}
	}

	return nil
}

func init() {
	if dir := getenvAny(EnvUserDataDir, LegacyEnvUserDataDir); dir != "" {
		Log.Debug("Using", Ternary(os.Getenv(EnvUserDataDir) != "", EnvUserDataDir, LegacyEnvUserDataDir))
		BaseDir = dir
	} else if dir = os.Getenv("DISCORD_USER_DATA_DIR"); dir != "" {
		Log.Debug("Using DISCORD_USER_DATA_DIR/../" + ClientDataDir)
		BaseDir = path.Join(dir, "..", ClientDataDir)
	} else {
		Log.Debug("Using UserConfig")
		BaseDir = appdir.New(BrandName).UserConfig()
	}
	dir := getenvAny(EnvInstallFile, LegacyEnvInstallFile)
	if dir == "" {
		if !ExistsFile(BaseDir) {
			BaseDirErr = os.Mkdir(BaseDir, 0755)
			if BaseDirErr != nil {
				Log.Error("Failed to create", BaseDir, BaseDirErr)
			} else {
				BaseDirErr = FixOwnership(BaseDir)
			}
		}
	}
	if dir != "" {
		Log.Debug("Using", Ternary(os.Getenv(EnvInstallFile) != "", EnvInstallFile, LegacyEnvInstallFile))
		KamidereDirectory = dir
	} else {
		KamidereDirectory = path.Join(BaseDir, ClientAsarFile)
		if !ExistsFile(KamidereDirectory) {
			legacyPath := path.Join(BaseDir, LegacyClientAsarFile)
			if _, err := os.Stat(legacyPath); err == nil {
				Log.Debug("Reusing legacy install at", legacyPath)
				KamidereDirectory = legacyPath
			} else if localBuildDir := findLocalDevelopmentBuild(); localBuildDir != "" {
				Log.Debug("Using local development build at", localBuildDir)
				KamidereDirectory = localBuildDir
			}
		}
	}
}

type DiscordInstall struct {
	path             string // the base path
	branch           string // canary / stable / ...
	appPath          string // List of app folder to patch
	isPatched        bool
	isFlatpak        bool
	isSystemElectron bool // Needs special care https://aur.archlinux.org/packages/discord_arch_electron
	isOpenAsar       *bool
}

//region Patch

func patchAppAsar(dir string, isSystemElectron bool) (err error) {
	appAsar := path.Join(dir, "app.asar")
	_appAsar := path.Join(dir, "_app.asar")

	var renamesDone [][]string
	defer func() {
		if err != nil && len(renamesDone) > 0 {
			Log.Error("Failed to patch. Undoing partial patch")
			for _, rename := range renamesDone {
				if innerErr := os.Rename(rename[1], rename[0]); innerErr != nil {
					Log.Error("Failed to undo partial patch. This install is probably bricked.", innerErr)
				} else {
					Log.Info("Successfully undid all changes")
				}
			}
		}
	}()

	Log.Debug("Renaming", appAsar, "to", _appAsar)
	if err := os.Rename(appAsar, _appAsar); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		return err
	}
	renamesDone = append(renamesDone, []string{appAsar, _appAsar})

	if isSystemElectron {
		from, to := appAsar+".unpacked", _appAsar+".unpacked"
		Log.Debug("Renaming", from, "to", to)
		err := os.Rename(from, to)
		if err != nil {
			return err
		}
		renamesDone = append(renamesDone, []string{from, to})
	}

	Log.Debug("Writing custom app.asar to", appAsar)
	if err := WriteAppAsar(appAsar, KamidereDirectory); err != nil {
		return err
	}

	return nil
}

func repairPatchedAppAsar(dir string, isSystemElectron bool) (err error) {
	appAsar := path.Join(dir, "app.asar")
	appAsarRepairTmp := path.Join(dir, "app.asar.repair.tmp")
	appAsarRepairBak := path.Join(dir, "app.asar.repair.bak")

	Log.Warn("Original backup is missing. Rewriting patched app.asar in place for repair.")

	_ = os.RemoveAll(appAsarRepairTmp)
	_ = os.RemoveAll(appAsarRepairBak)

	if err = WriteAppAsar(appAsarRepairTmp, KamidereDirectory); err != nil {
		return err
	}

	restoreOriginal := false
	defer func() {
		if err != nil {
			_ = os.RemoveAll(appAsarRepairTmp)
			if restoreOriginal {
				if innerErr := os.Rename(appAsarRepairBak, appAsar); innerErr != nil {
					Log.Error("Failed to restore app.asar after repair failure.", innerErr)
				}
			}
			return
		}

		_ = os.RemoveAll(appAsarRepairTmp)
		_ = os.RemoveAll(appAsarRepairBak)
	}()

	if pathExists(appAsar) {
		if err = os.Rename(appAsar, appAsarRepairBak); err != nil {
			err = CheckIfErrIsCauseItsBusyRn(err)
			return err
		}
		restoreOriginal = true
	}

	if err = os.Rename(appAsarRepairTmp, appAsar); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		return err
	}

	if isSystemElectron && pathExists(appAsarRepairBak+".unpacked") && !pathExists(appAsar+".unpacked") {
		if err = os.Rename(appAsarRepairBak+".unpacked", appAsar+".unpacked"); err != nil {
			return err
		}
	}

	return nil
}

func (di *DiscordInstall) patch() error {
	Log.Info("Patching " + di.path + "...")
	if LatestHash != InstalledHash {
		if err := InstallLatestBuilds(); err != nil {
			return nil // already shown dialog so don't return same error again
		}
	}
	if err := validateKamiderePayload(); err != nil {
		return err
	}

	PreparePatch(di)

	if di.isPatched {
		Log.Info(di.path, "is already patched. Unpatching first...")
		if err := di.unpatch(); err != nil {
			if errors.Is(err, ErrOriginalBackupMissing) {
				Log.Warn("Original backup is missing. Continuing with in-place repair instead of full unpatch.")
			} else if errors.Is(err, os.ErrPermission) {
				return err
			} else {
				return errors.New("patch: Failed to unpatch already patched install '" + di.path + "':\n" + err.Error())
			}
		}
	}

	targetDir := path.Join(di.appPath, "..")
	if di.isSystemElectron {
		targetDir = di.path
	}

	if di.isPatched && !pathExists(path.Join(targetDir, "_app.asar")) {
		if err := repairPatchedAppAsar(targetDir, di.isSystemElectron); err != nil {
			return err
		}
	} else {
		if err := patchAppAsar(targetDir, di.isSystemElectron); err != nil {
			return err
		}
	}

	Log.Info("Successfully patched", di.path)
	di.isPatched = true

	if di.isFlatpak {
		pathElements := strings.Split(di.path, "/")
		var name string
		for _, e := range pathElements {
			if strings.HasPrefix(e, "com.discordapp") {
				name = e
				break
			}
		}

		Log.Debug("This is a flatpak. Trying to grant the Flatpak access to", KamidereDirectory+"...")

		isSystemFlatpak := strings.HasPrefix(di.path, "/var")
		var args []string
		if !isSystemFlatpak {
			args = append(args, "--user")
		}
		args = append(args, "override", name, "--filesystem="+KamidereDirectory)
		fullCmd := "flatpak " + strings.Join(args, " ")

		Log.Debug("Running", fullCmd)

		var err error
		if !isSystemFlatpak && os.Getuid() == 0 {
			// We are operating on a user flatpak but are root
			actualUser := os.Getenv("SUDO_USER")
			Log.Debug("This is a user install but we are root. Using su to run as", actualUser)
			cmd := exec.Command("su", "-", actualUser, "-c", "sh", "-c", fullCmd)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		} else {
			cmd := exec.Command("flatpak", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		}
		if err != nil {
			return errors.New("Failed to grant Discord Flatpak access to " + KamidereDirectory + ": " + err.Error())
		}
	}
	return nil
}

//endregion

// region Unpatch

func unpatchAppAsar(dir string, isSystemElectron bool) (errOut error) {
	appAsar := path.Join(dir, "app.asar")
	appAsarTmp := path.Join(dir, "app.asar.tmp")
	_appAsar := path.Join(dir, "_app.asar")

	if !pathExists(_appAsar) {
		return fmt.Errorf("%w at %s", ErrOriginalBackupMissing, _appAsar)
	}

	var renamesDone [][]string
	defer func() {
		if errOut != nil && len(renamesDone) > 0 {
			Log.Error("Failed to unpatch. Undoing partial unpatch")
			for _, rename := range renamesDone {
				if innerErr := os.Rename(rename[1], rename[0]); innerErr != nil {
					Log.Error("Failed to undo partial unpatch. This install is probably bricked.", innerErr)
				} else {
					Log.Info("Successfully undid all changes")
				}
			}
		} else if errOut == nil {
			if innerErr := os.RemoveAll(appAsarTmp); innerErr != nil {
				Log.Warn("Failed to delete temporary app.asar (patch folder) backup. This is whatever but you might want to delete it manually.", innerErr)
			}
		}
	}()

	Log.Debug("Deleting", appAsar)
	if err := os.Rename(appAsar, appAsarTmp); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		errOut = err
	} else {
		renamesDone = append(renamesDone, []string{appAsar, appAsarTmp})
	}

	Log.Debug("Renaming", _appAsar, "to", appAsar)
	if err := os.Rename(_appAsar, appAsar); err != nil {
		err = CheckIfErrIsCauseItsBusyRn(err)
		Log.Error(err.Error())
		errOut = err
	} else {
		renamesDone = append(renamesDone, []string{_appAsar, appAsar})
	}

	if isSystemElectron {
		Log.Debug("Renaming", _appAsar+".unpacked", "to", appAsar+".unpacked")
		if err := os.Rename(_appAsar+".unpacked", appAsar+".unpacked"); err != nil {
			Log.Error(err.Error())
			errOut = err
		}
	}
	return
}

func (di *DiscordInstall) unpatch() error {
	Log.Info("Unpatching " + di.path + "...")

	PreparePatch(di)

	if di.isSystemElectron {
		if err := unpatchAppAsar(di.path, true); err != nil {
			return err
		}
	} else {
		if err := unpatchAppAsar(path.Join(di.appPath, ".."), false); err != nil {
			return err
		}
	}

	Log.Info("Successfully unpatched", di.path)
	di.isPatched = false
	return nil
}

//endregion
