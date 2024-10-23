#define MyAppName "FMU-API"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Your Company"
#define MyAppExeName "fmu-api.exe"

[Setup]
AppId={{YOUR-GUID-HERE}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={pf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
OutputDir=output
OutputBaseFilename=fmu-api-setup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin

[Languages]
;Name: "english"; MessagesFile: "compiler:Languages\English.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "installservice"; Description: "Install as a Windows service"; GroupDescription: "Additional tasks:"

[Files]
; Main program files
Source: "for_build\fmu\fmu-api.exe"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs
Source: "for_build\fmu\wwwroot\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs
;Source: "x86 full\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{commondesktop}\{#MyAppName}"; Filename: "C:\Windows\system32\cmd.exe"; Parameters: "/C start http://localhost:2578/"; Tasks: desktopicon; WorkingDir: "C:\Program Files\Automation\FMU-API"

[Run]
; Install service if the corresponding option is selected
Filename: "{app}\{#MyAppExeName}"; Parameters: "--install"; Tasks: installservice; Flags: runhidden waituntilterminated; StatusMsg: "Installing Windows Service..."
; Wait for 5 seconds before starting the service
Filename: "{sys}\cmd.exe"; Parameters: "/C timeout /T 5 /NOBREAK > ""{app}\install_log.txt"" 2>&1"; Flags: runhidden waituntilterminated; StatusMsg: "Waiting for 5 seconds..."
; Start the service after the pause
Filename: "{sys}\sc.exe"; Parameters: "start FMU-API >> ""{app}\install_log.txt"" 2>&1"; Flags: runhidden waituntilterminated; StatusMsg: "Starting Windows Service..."

[UninstallRun]
; Remove service on uninstallation
Filename: "{app}\{#MyAppExeName}"; Parameters: "--uninstall"; Flags: runhidden waituntilterminated; StatusMsg: "Removing Windows Service..."
; Delete all files and subfolders in C:\Program Files\Automation if it exists
Filename: "{sys}\cmd.exe"; Parameters: "/C rmdir /S /Q ""C:\Program Files\Automation"""; Flags: runhidden waituntilterminated; StatusMsg: "Deleting Automation files..."

[Code]
var
  ErrorCode: Integer;

// Check if CouchDB is installed
function IsCouchDBInstalled: Boolean;
begin
  Result := FileExists(ExpandConstant('{pf}\Apache CouchDB\bin\couchdb.exe')) or
            FileExists(ExpandConstant('{pf32}\Apache CouchDB\bin\couchdb.exe'));
end;

// Check before installation
function InitializeSetup(): Boolean;
begin
  Result := True;
  
  if not IsCouchDBInstalled then
  begin
    if MsgBox('FMU-API requires Apache CouchDB to work. Would you like to open the download page?',
      mbConfirmation, MB_YESNO) = IDYES then
    begin
      ShellExec('open', 'https://couchdb.apache.org/downloads.html', '', '', SW_SHOW, ewNoWait, ErrorCode);
    end;
    
    if MsgBox('Continue installation without CouchDB?', mbConfirmation, MB_YESNO) = IDNO then
    begin
      Result := False;
    end;
  end;
end;

// Actions after installation
procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then 
  begin
    if IsTaskSelected('installservice') then
    begin
      // Check if the service was installed
      if not Exec(ExpandConstant('{sys}\sc.exe'), 'query "FMU-API"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
      begin
        MsgBox('Error checking service installation. Code: ' + IntToStr(ResultCode), mbError, MB_OK);
      end
      else if ResultCode <> 0 then
      begin
        MsgBox('The service was not installed correctly. Check administrator rights.', mbError, MB_OK);
      end;
    end;
  end;
end;
