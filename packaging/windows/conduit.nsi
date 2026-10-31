Unicode true
ManifestDPIAware true

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "nsDialogs.nsh"

!insertmacro GetParameters
!insertmacro GetOptions

; The build scripts pass the version from daemon/version/VERSION:
;   makensis /DVERSION=1.3.0 /DVERSION_QUAD=1.3.0.0 conduit.nsi
; This file never holds its own copy.
!ifndef VERSION
  !error "Pass the version: makensis /DVERSION=x.y.z /DVERSION_QUAD=x.y.z.0 conduit.nsi (scripts/build_windows.ps1 does this)"
!endif
!ifndef VERSION_QUAD
  !error "Pass VERSION_QUAD, the four-part numeric form of VERSION (x.y.z.0)"
!endif

; File properties of the setup executable.
VIProductVersion "${VERSION_QUAD}"
VIFileVersion "${VERSION_QUAD}"
VIAddVersionKey "ProductName" "Conduit"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "Conduit Setup"
VIAddVersionKey "CompanyName" "Conduit"
VIAddVersionKey "LegalCopyright" "GPL-3.0-or-later"

Var ExistingVersion
Var ExistingDir
Var CleanInstall
Var RadioUpdate
Var RadioClean
Var SkipAria2

; The aria2 download engine is pinned in scripts/aria2_binaries.env. Conduit
; does not ship it: the installer downloads the pinned build, and the user can
; place aria2c.exe by hand if that fails.
!ifndef ARIA2_WIN_URL
  !error "Pass ARIA2_WIN_URL and ARIA2_WIN_SHA256 (scripts/build_windows.ps1 does this)"
!endif
!ifndef ARIA2_WIN_SHA256
  !error "Pass ARIA2_WIN_SHA256 (scripts/build_windows.ps1 does this)"
!endif

!ifndef EXE
  !define EXE "..\..\desktop\build\bin\Conduit.exe"
!endif
!ifndef EXTDIR
  !define EXTDIR "..\..\extension"
!endif
!ifndef ICON
  !define ICON "..\..\desktop\icon.ico"
!endif

Name "Conduit"
OutFile "..\..\dist\Conduit-windows-amd64-setup.exe"
InstallDir "$LOCALAPPDATA\Programs\Conduit"
InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "InstallLocation"
RequestExecutionLevel user
ShowInstDetails show
ShowUninstDetails show

!define MUI_ABORTWARNING
!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"
Page custom ExistingInstallPage ExistingInstallLeave
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
; Launch is checked by default; the notices box below stays unchecked.
!define MUI_FINISHPAGE_RUN "$INSTDIR\Conduit.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Conduit"
!define MUI_FINISHPAGE_SHOWREADME "$INSTDIR\THIRD_PARTY_NOTICES.md"
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Show third-party notices"
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Conduit" SecApp
  SectionIn RO
  SetShellVarContext current

  ; Stop the old instance before replacing files. --shutdown exists in this
  ; version; taskkill is the upgrade fallback for older builds.
  Call StopConduit

  ${If} $CleanInstall == "1"
    Call CleanPreviousInstall
  ${EndIf}

  ; Wails used to place the WebView2 profile next to the executable name.
  ; The new build uses %LOCALAPPDATA%\conduit\webview, so remove the legacy
  ; cache during an upgrade as well as on uninstall.
  RMDir /r "$APPDATA\Conduit.exe"
  RMDir /r "$APPDATA\Conduit.exe.cleanup"

  RMDir /r "$INSTDIR\extension"
  Delete "$INSTDIR\Conduit.exe"
  Delete "$INSTDIR\uninstall.exe"
  IfFileExists "$INSTDIR\Conduit.exe" 0 InstallConduitFileGone
  MessageBox MB_OK|MB_ICONSTOP "Conduit is still running and could not be stopped. Quit it from the tray and run the installer again." /SD IDOK
  Abort
  InstallConduitFileGone:

  Call EnsureWebView2
  Call EnsureAria2

  SetOutPath "$INSTDIR"
  File "/oname=Conduit.exe" "${EXE}"
  SetOutPath "$INSTDIR\extension"
  File /r /x test "${EXTDIR}\*"
  SetOutPath "$INSTDIR"
  File "..\..\LICENSE"
  File "..\..\NOTICE"
  File "..\..\THIRD_PARTY_NOTICES.md"
  File "..\..\SOURCE_OFFER.md"
  SetOutPath "$INSTDIR\licenses"
  File /r "..\..\licenses\*"
  SetOutPath "$INSTDIR"
  CreateDirectory "$SMPROGRAMS\Conduit"
  CreateShortcut "$SMPROGRAMS\Conduit\Conduit.lnk" "$INSTDIR\Conduit.exe"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "DisplayName" "Conduit"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "DisplayIcon" "$INSTDIR\Conduit.exe"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "Publisher" "Conduit"
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "NoRepair" 1

  ; Best-effort: stop any engine left by the previous build. Keep mode preserves
  ; config.json and aria2.session. The engine under $INSTDIR\engine is kept.
  ExecWait '"$INSTDIR\Conduit.exe" --uninstall-cleanup=keep' $0
SectionEnd

Section "Desktop shortcut" SecDesktop
  CreateShortcut "$DESKTOP\Conduit.lnk" "$INSTDIR\Conduit.exe"
SectionEnd

Section "Uninstall"
  SetShellVarContext current

  ; Deep clean is an explicit choice, not the default. The warning is shown
  ; before the app is stopped or any Conduit file is removed.
  StrCpy $1 "keep"
  MessageBox MB_YESNO|MB_ICONEXCLAMATION "Deep clean will:$\r$\n$\r$\n- Stop all Conduit download engines (aria2c), including leftovers from previous launches.$\r$\n- Delete the aria2c.exe copy under %AppData%\Conduit\bin, if one was placed there.$\r$\n- Delete all Conduit settings, download session data, and cached app data.$\r$\n$\r$\nIn-progress Conduit downloads will be interrupted. Downloaded files and partial downloads are kept. Other applications' aria2c processes are not affected.$\r$\n$\r$\nChoose Yes for deep clean, or No to keep your settings for a future reinstall." /SD IDNO IDNO NormalUninstall
  StrCpy $1 "purge"
  Goto CleanupModeReady

  NormalUninstall:
  MessageBox MB_YESNO|MB_ICONQUESTION "Keep your Conduit settings and download session for a future reinstall?$\r$\n$\r$\nChoose No to remove them. Downloaded files are always kept." /SD IDYES IDYES CleanupModeReady
  StrCpy $1 "purge"
  CleanupModeReady:

  ; Every uninstall stops the app and the engine. The helper verifies that it
  ; is stopping Conduit-owned processes only.
  Call un.StopConduit

  IfFileExists "$INSTDIR\Conduit.exe" 0 UninstallHelperDone
  ExecWait '"$INSTDIR\Conduit.exe" --uninstall-cleanup=$1' $2
  ${If} $2 != 0
    MessageBox MB_OK|MB_ICONEXCLAMATION "Some Conduit runtime files were in use. They have been scheduled for removal after the next Windows restart." /SD IDOK
  ${EndIf}
  UninstallHelperDone:

  Delete "$DESKTOP\Conduit.lnk"
  RMDir /r "$SMPROGRAMS\Conduit"
  RMDir /r "$INSTDIR\extension"
  Delete /REBOOTOK "$INSTDIR\Conduit.exe"
  Delete /REBOOTOK "$INSTDIR\uninstall.exe"
  ; Remove only what this installer places. A recursive delete of $INSTDIR
  ; would also take the user's own files if uninstall.exe ran from another folder.
  Delete /REBOOTOK "$INSTDIR\LICENSE"
  Delete /REBOOTOK "$INSTDIR\NOTICE"
  Delete /REBOOTOK "$INSTDIR\THIRD_PARTY_NOTICES.md"
  Delete /REBOOTOK "$INSTDIR\SOURCE_OFFER.md"
  RMDir /r /REBOOTOK "$INSTDIR\licenses"
  RMDir /r /REBOOTOK "$INSTDIR\engine"
  RMDir "$INSTDIR"

  ; The helper normally removes these too; these are the fallback paths if
  ; Conduit.exe was already missing or cleanup partially failed.
  Delete /REBOOTOK "$APPDATA\Conduit\engine.json"
  Delete /REBOOTOK "$APPDATA\Conduit\engine.json.tmp"
  Delete /REBOOTOK "$APPDATA\Conduit\instance.lock"
  Delete /REBOOTOK "$APPDATA\Conduit\config.json.tmp"
  RMDir /r /REBOOTOK "$APPDATA\Conduit\bin"
  RMDir /r /REBOOTOK "$LOCALAPPDATA\conduit"
  RMDir /r /REBOOTOK "$APPDATA\Conduit.exe"
  RMDir /r /REBOOTOK "$APPDATA\Conduit.exe.cleanup"
  ${If} $1 == "purge"
    RMDir /r /REBOOTOK "$APPDATA\Conduit"
  ${EndIf}

  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "Conduit"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run" "Conduit"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run32" "Conduit"
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit"

  IfSilent ReminderDone
  MessageBox MB_OK|MB_ICONINFORMATION "The Conduit browser extension is managed by your browser and is not removed automatically. Remove it from your browser's extensions page if you no longer need it." /SD IDOK
  ReminderDone:
SectionEnd

; .onInit looks for an earlier install. /CLEAN on the command line selects a
; clean install for silent runs; silent runs otherwise update in place.
; /NOARIA2 skips the aria2 download. The engine can be placed by hand later.
Function .onInit
  StrCpy $CleanInstall "0"
  StrCpy $SkipAria2 "0"
  ReadRegStr $ExistingDir HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "InstallLocation"
  ReadRegStr $ExistingVersion HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit" "DisplayVersion"
  ${If} $ExistingDir != ""
    IfFileExists "$ExistingDir\Conduit.exe" ExistingFound
    ; The registry entry outlived the program, so treat this as a first install.
    StrCpy $ExistingDir ""
    ExistingFound:
  ${EndIf}
  ${If} $ExistingVersion == ""
    StrCpy $ExistingVersion "an earlier version"
  ${EndIf}

  ${GetParameters} $R0
  ${GetOptions} $R0 "/CLEAN" $R1
  ${IfNot} ${Errors}
    StrCpy $CleanInstall "1"
  ${EndIf}

  ${GetOptions} $R0 "/NOARIA2" $R1
  ${IfNot} ${Errors}
    StrCpy $SkipAria2 "1"
  ${EndIf}
FunctionEnd

Function ExistingInstallPage
  ${If} $ExistingDir == ""
    Abort
  ${EndIf}
  !insertmacro MUI_HEADER_TEXT "Existing installation found" "Choose how to install Conduit ${VERSION}."
  nsDialogs::Create 1018
  Pop $0
  ${NSD_CreateLabel} 0 0 100% 28u "Conduit $ExistingVersion is installed in:$\r$\n$ExistingDir"
  Pop $0
  ${NSD_CreateRadioButton} 0 38u 100% 12u "Update the existing installation (recommended)"
  Pop $RadioUpdate
  ${NSD_AddStyle} $RadioUpdate ${WS_GROUP}
  ${NSD_CreateLabel} 12u 52u 95% 20u "Keeps your settings, download queue, and site exclusions."
  Pop $0
  ${NSD_CreateRadioButton} 0 80u 100% 12u "Clean install"
  Pop $RadioClean
  ${NSD_CreateLabel} 12u 94u 95% 30u "Removes Conduit settings, the download queue, and cached data first. Files you already downloaded are kept."
  Pop $0
  ${If} $CleanInstall == "1"
    ${NSD_Check} $RadioClean
  ${Else}
    ${NSD_Check} $RadioUpdate
  ${EndIf}
  nsDialogs::Show
FunctionEnd

Function ExistingInstallLeave
  ${NSD_GetState} $RadioClean $0
  ${If} $0 == ${BST_CHECKED}
    MessageBox MB_YESNO|MB_ICONEXCLAMATION "A clean install deletes your Conduit settings and download queue. Downloaded files are kept.$\r$\n$\r$\nContinue?" IDYES CleanConfirmed
    Abort
    CleanConfirmed:
    StrCpy $CleanInstall "1"
  ${Else}
    StrCpy $CleanInstall "0"
  ${EndIf}
FunctionEnd

; Runs the previous build's helper in purge mode, then clears what it may have
; missed. Downloaded files are never touched.
Function CleanPreviousInstall
  DetailPrint "Removing previous settings and data..."
  IfFileExists "$ExistingDir\Conduit.exe" 0 CleanHelperDone
  ExecWait '"$ExistingDir\Conduit.exe" --uninstall-cleanup=purge' $0
  CleanHelperDone:
  RMDir /r "$APPDATA\Conduit"
  RMDir /r "$LOCALAPPDATA\conduit"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "Conduit"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run" "Conduit"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run32" "Conduit"
FunctionEnd

Function StopConduit
  IfFileExists "$INSTDIR\Conduit.exe" 0 StopDone
  DetailPrint "Stopping Conduit..."
  ExecWait '"$INSTDIR\Conduit.exe" --shutdown' $0
  Sleep 800
  nsExec::ExecToLog 'taskkill /F /IM Conduit.exe'
  Pop $0
  Sleep 300
  StopDone:
FunctionEnd

Function un.StopConduit
  IfFileExists "$INSTDIR\Conduit.exe" 0 un.StopDone
  DetailPrint "Stopping Conduit..."
  ExecWait '"$INSTDIR\Conduit.exe" --shutdown' $0
  Sleep 800
  nsExec::ExecToLog 'taskkill /F /IM Conduit.exe'
  Pop $0
  Sleep 300
  un.StopDone:
FunctionEnd

; The download engine comes from the pinned upstream release. aria2.ps1 checks
; its SHA-256. A failure is not fatal: Conduit still installs and reports the
; engine as missing until the user places aria2c.exe by hand.
Function EnsureAria2
  ${If} $SkipAria2 == "1"
    DetailPrint "Skipping the aria2 download engine (/NOARIA2)."
    Return
  ${EndIf}
  DetailPrint "Installing the aria2 download engine"
  InitPluginsDir
  File "/oname=$PLUGINSDIR\aria2.ps1" "aria2.ps1"
  ExecWait 'powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\aria2.ps1" -Url "${ARIA2_WIN_URL}" -Sha256 "${ARIA2_WIN_SHA256}" -Destination "$INSTDIR\engine\aria2c.exe"' $0
  ${If} $0 != 0
    Call Aria2ManualNotice
  ${EndIf}
FunctionEnd

Function Aria2ManualNotice
  MessageBox MB_OK|MB_ICONEXCLAMATION "Conduit could not install its download engine (aria2c), so downloads will not start yet.$\r$\n$\r$\nTo fix it, download aria2 1.37.0 from github.com/aria2/aria2/releases, copy aria2c.exe to $INSTDIR\engine\, and restart Conduit." /SD IDOK
FunctionEnd

Function EnsureWebView2
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 == ""
    ReadRegStr $0 HKCU "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${EndIf}
  ${If} $0 == ""
    DetailPrint "Installing the Microsoft Edge WebView2 Runtime"
    InitPluginsDir
    File "/oname=$PLUGINSDIR\webview2.ps1" "webview2.ps1"
    ExecWait 'powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\webview2.ps1"' $0
    ${If} $0 != 0
      Call WebView2ManualNotice
    ${EndIf}
  ${EndIf}
FunctionEnd

; Without WebView2 the Conduit window cannot open, so the user needs to know.
Function WebView2ManualNotice
  MessageBox MB_OK|MB_ICONEXCLAMATION "Conduit could not install the Microsoft Edge WebView2 Runtime, so the Conduit window cannot open.$\r$\n$\r$\nInstall the Evergreen Runtime from https://developer.microsoft.com/microsoft-edge/webview2/ and start Conduit again." /SD IDOK
FunctionEnd

LangString DESC_App ${LANG_ENGLISH} "Conduit application and browser extension files."
LangString DESC_Desktop ${LANG_ENGLISH} "Add a Conduit shortcut to the desktop."
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecApp} $(DESC_App)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} $(DESC_Desktop)
!insertmacro MUI_FUNCTION_DESCRIPTION_END
