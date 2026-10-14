[Setup]
AppId={{A7B3E2C1-5D44-4F1A-9C3E-8B2D6F0A1E77}
AppName=shapez-auto
AppVersion=0.1.6
SetupIconFile=..\assets\icon.ico
UninstallDisplayIcon={app}\shapez-auto.exe
AppPublisher=shapez-auto
DefaultDirName={localappdata}\Programs\shapez-auto
DefaultGroupName=shapez-auto
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=shapez-auto-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest
WizardStyle=modern

[Languages]
Name: "default"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\dist\shapez-auto.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\assets\shapez-auto.js"; DestDir: "{app}\mod"; Flags: ignoreversion

[Icons]
Name: "{group}\shapez-auto"; Filename: "{app}\shapez-auto.exe"
Name: "{autodesktop}\shapez-auto"; Filename: "{app}\shapez-auto.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加选项:"; Flags: unchecked

[Run]
Filename: "{app}\shapez-auto.exe"; Description: "启动 shapez-auto"; Flags: nowait postinstall skipifsilent
