; SmartEYE installer (Inno Setup 6+).
; One installer, two roles: the user picks Server (o'qituvchi/rahbar) or
; Client (o'quvchi/xodim) on a custom wizard page, and — for a client — types
; the network code shown on the server. The role is written to config via
; `smarteye.exe --setup`, and a client is registered to auto-start at logon.
;
; Build:  iscc /DAppVersion=1.0.0 smarteye.iss
; Expects smarteye.exe next to this script (copied by the build workflow).

#ifndef AppVersion
  #define AppVersion "1.0.0"
#endif

[Setup]
AppId={{B8E7B4F2-7C3A-4E1D-9A2B-5E6F7A8B9C0D}
AppName=SmartEYE
AppVersion={#AppVersion}
AppPublisher=SmartEYE
DefaultDirName={autopf}\SmartEYE
DefaultGroupName=SmartEYE
DisableProgramGroupPage=yes
OutputBaseFilename=SmartEYE-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=.

[Languages]
Name: "uz"; MessagesFile: "compiler:Default.isl"
Name: "ru"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "en"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "smarteye.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.txt"; DestDir: "{app}"; Flags: ignoreversion isreadme skipifsourcedoesntexist

[Icons]
; Server gets a desktop + start-menu shortcut to open the control panel.
Name: "{group}\SmartEYE Boshqaruv paneli"; Filename: "{app}\smarteye.exe"; Check: IsServer
Name: "{autodesktop}\SmartEYE"; Filename: "{app}\smarteye.exe"; Check: IsServer
Name: "{group}\SmartEYE'ni o'chirish"; Filename: "{uninstallexe}"

[Registry]
; Client auto-starts at logon in the interactive desktop session (required so
; it can show the real screen and accept remote control).
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
  ValueType: string; ValueName: "SmartEYE"; ValueData: """{app}\smarteye.exe"""; \
  Flags: uninsdeletevalue; Check: IsClient

[Run]
; Apply the chosen configuration right after files are copied.
Filename: "{app}\smarteye.exe"; Parameters: "{code:SetupArgs}"; Flags: runhidden waituntilterminated
; Launch now: the server opens its dashboard; the client starts in background.
Filename: "{app}\smarteye.exe"; Description: "SmartEYE'ni hoziroq ishga tushirish"; \
  Flags: nowait postinstall skipifsilent

[UninstallRun]
; Stop any running instance on uninstall.
Filename: "{sys}\taskkill.exe"; Parameters: "/F /IM smarteye.exe"; Flags: runhidden; RunOnceId: "KillSmartEYE"

[Code]
var
  RolePage: TInputOptionWizardPage;
  CodePage: TInputQueryWizardPage;
  GeneratedCode: String;

function IsServer: Boolean;
begin
  Result := (RolePage.SelectedValueIndex = 0);
end;

function IsClient: Boolean;
begin
  Result := (RolePage.SelectedValueIndex = 1);
end;

// Make a random 6-digit code for a new server.
function MakeCode: String;
var
  n: Longint;
begin
  n := Random(1000000);
  Result := Format('%.6d', [n]);
end;

procedure InitializeWizard;
begin
  Randomize;
  GeneratedCode := MakeCode;

  RolePage := CreateInputOptionPage(wpSelectDir,
    'O''rnatish turi', 'Bu kompyuter qanday vazifani bajaradi?',
    'Iltimos, rolni tanlang. Keyinchalik qayta o''rnatish orqali o''zgartirish mumkin.',
    True, False);
  RolePage.Add('Server — boshqaruvchi (o''qituvchi / rahbar). Bu kompyuterdan boshqalar kuzatiladi.');
  RolePage.Add('Client — kuzatiladigan (o''quvchi / xodim). Bu kompyuter fon rejimida ishlaydi.');
  RolePage.SelectedValueIndex := 0;

  CodePage := CreateInputQueryPage(RolePage.ID,
    'Tarmoq kodi', 'Server va clientlarni bog''laydigan maxfiy kod',
    'Server uchun avtomatik kod yaratildi. Client o''rnatganda, serverda ko''rsatilgan shu kodni kiriting.');
  CodePage.Add('Tarmoq kodi:', False);
  CodePage.Add('Server nomi (ixtiyoriy, masalan "3-xona"):', False);
  CodePage.Values[0] := GeneratedCode;
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  // When arriving at the code page, pre-fill a fresh code for servers and clear
  // it for clients (who must type the server's code).
  if CurPageID = CodePage.ID then
  begin
    if IsServer then
    begin
      CodePage.Values[0] := GeneratedCode;
      CodePage.SubCaptionLabel.Caption :=
        'Bu server uchun kod: ' + GeneratedCode + '. Uni yozib oling — clientlarga shu kod kerak bo''ladi.';
    end
    else
    begin
      CodePage.Values[0] := '';
      CodePage.SubCaptionLabel.Caption :=
        'Serverda ko''rsatilgan tarmoq kodini kiriting.';
    end;
  end;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID = CodePage.ID) then
  begin
    if Trim(CodePage.Values[0]) = '' then
    begin
      MsgBox('Iltimos, tarmoq kodini kiriting.', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

// Build the argument string for `smarteye.exe --setup`.
function SetupArgs(Param: String): String;
var
  role, code, name: String;
begin
  if IsServer then role := 'server' else role := 'client';
  code := Trim(CodePage.Values[0]);
  name := Trim(CodePage.Values[1]);
  Result := '--setup --role ' + role + ' --code "' + code + '"';
  if IsServer and (name <> '') then
    Result := Result + ' --name "' + name + '"';
end;
