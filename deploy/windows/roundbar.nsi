; Roundbar for Windows: per-user installer, no administrator rights needed.
;
;   go run ./tools/winicon -o deploy/windows/roundbar.ico
;   makensis -DVERSION=1.2.3 -DEXE=/path/roundbar.exe -DOUTFILE=/path/roundbar-setup-windows-amd64.exe deploy/windows/roundbar.nsi
;
; Installs roundbar.exe to %LOCALAPPDATA%\Programs\Roundbar, adds a Start menu
; shortcut, starts the tray app at login (HKCU Run key) and registers an
; uninstaller in Apps & features. The configuration (%APPDATA%\roundbar) is
; left in place on uninstall.

Unicode true
SetCompressor /SOLID lzma

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef VERSION4
  !define VERSION4 "0.0.0.0"
!endif
!ifndef EXE
  !define EXE "roundbar.exe"
!endif
!ifndef OUTFILE
  !define OUTFILE "roundbar-setup-windows-amd64.exe"
!endif

!define APP "Roundbar"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Roundbar"
!define RUN_KEY "Software\Microsoft\Windows\CurrentVersion\Run"

Name "${APP}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\Roundbar"
InstallDirRegKey HKCU "Software\Roundbar" "InstallDir"
RequestExecutionLevel user
ShowInstDetails nevershow
BrandingText "${APP} ${VERSION}"

VIProductVersion "${VERSION4}"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "${APP} setup"
VIAddVersionKey "LegalCopyright" "MIT License"

!include "MUI2.nsh"
!include "FileFunc.nsh"

!define MUI_ICON "roundbar.ico"
!define MUI_UNICON "roundbar.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\roundbar.exe"
!define MUI_FINISHPAGE_RUN_TEXT "$(StartNow)"

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "French"

LangString StartNow ${LANG_ENGLISH} "Start Roundbar"
LangString StartNow ${LANG_FRENCH} "Lancer Roundbar"

; A running copy holds the exe open: stop it before replacing or removing it.
!macro StopRunning
  nsExec::Exec '"$SYSDIR\taskkill.exe" /F /IM roundbar.exe'
  Pop $0
  Sleep 800
!macroend

Section "Roundbar"
  !insertmacro StopRunning
  SetOutPath "$INSTDIR"
  File "/oname=roundbar.exe" "${EXE}"
  File "roundbar.ico"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateShortCut "$SMPROGRAMS\Roundbar.lnk" "$INSTDIR\roundbar.exe" "" "$INSTDIR\roundbar.ico" 0
  WriteRegStr HKCU "${RUN_KEY}" "Roundbar" '"$INSTDIR\roundbar.exe" tray'
  WriteRegStr HKCU "Software\Roundbar" "InstallDir" "$INSTDIR"

  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${APP}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${APP}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\roundbar.ico"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"

  ; The finish page offers to start it; a silent install (/S) starts it right away.
  IfSilent 0 +2
    Exec '"$INSTDIR\roundbar.exe" tray'
SectionEnd

Section "Uninstall"
  !insertmacro StopRunning
  Delete "$INSTDIR\roundbar.exe"
  Delete "$INSTDIR\roundbar.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\Roundbar.lnk"
  DeleteRegValue HKCU "${RUN_KEY}" "Roundbar"
  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\Roundbar"
  ; the daemon's socket and log; the configuration in %APPDATA%\roundbar stays
  RMDir /r "$LOCALAPPDATA\roundbar"
SectionEnd
