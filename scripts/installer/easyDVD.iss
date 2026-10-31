; Build through scripts/build-installer.ps1 from a hash-verified runtime package.
#ifndef BundleDir
  #error BundleDir is required
#endif
#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef InstallerOutputDir
  #error InstallerOutputDir is required
#endif
#ifdef InstallerTest
  #define ProductId "easyDVD.InstallerTest"
  #define ProductName "easyDVD Installer Test"
  #define OutputSuffix "-test"
#else
  ; Keep this identity and the install directory stable across releases.
  #define ProductId "easyDVD"
  #define ProductName "easyDVD"
  #define OutputSuffix ""
#endif

[Setup]
AppId={#ProductId}
AppName={#ProductName}
AppVersion={#AppVersion}
AppPublisher=easyDVD contributors
AppPublisherURL=https://github.com/maghofer/easyDVD
AppSupportURL=https://github.com/maghofer/easyDVD/issues
AppUpdatesURL=https://github.com/maghofer/easyDVD/releases
DefaultDirName={localappdata}\Programs\{#ProductName}
DefaultGroupName={#ProductName}
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#InstallerOutputDir}
OutputBaseFilename=easyDVD-{#AppVersion}-win64-setup{#OutputSuffix}
SetupIconFile=..\..\internal\ui\assets\app.ico
UninstallDisplayIcon={app}\easyDVD.exe
UninstallDisplayName={#ProductName}
LicenseFile={#BundleDir}\LICENSE
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
DisableProgramGroupPage=yes
DisableWelcomePage=no
UsePreviousAppDir=yes
UsePreviousTasks=yes
AppMutex=Global\easyDVD.Running
SetupMutex=Global\{#ProductId}.Setup
; Never use Restart Manager to close an encoder or force a reboot.
CloseApplications=no
RestartApplications=no
RestartIfNeededByRun=no
UninstallLogMode=append

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "{#BundleDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{userprograms}\{#ProductName}\{#ProductName}"; Filename: "{app}\easyDVD.exe"; WorkingDir: "{app}"
Name: "{userdesktop}\{#ProductName}"; Filename: "{app}\easyDVD.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\easyDVD.exe"; Description: "Open {#ProductName}"; Flags: nowait postinstall skipifsilent

; Intentionally no [UninstallDelete] rules: user data, private FFmpeg downloads,
; output, work directories and files not installed by Setup must be preserved.
