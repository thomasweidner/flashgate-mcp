function New-WindowsConfigFixture {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$Document
    )

    # An elevated token may default new files to a group owner. Set the
    # exact token user and its protected DACL when creating the fixture.
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    try {
        $user = $identity.User
        $security = [Security.AccessControl.FileSecurity]::new()
        $security.SetOwner($user)
        $security.SetAccessRuleProtection($true, $false)
        $rule = [Security.AccessControl.FileSystemAccessRule]::new(
            $user,
            [Security.AccessControl.FileSystemRights]::FullControl,
            [Security.AccessControl.AccessControlType]::Allow
        )
        $security.AddAccessRule($rule)
        $stream = [IO.FileSystemAclExtensions]::Create(
            [IO.FileInfo]::new($Path),
            [IO.FileMode]::CreateNew,
            [Security.AccessControl.FileSystemRights]::Write,
            [IO.FileShare]::None,
            4096,
            [IO.FileOptions]::None,
            $security
        )
        try {
            $bytes = [Text.UTF8Encoding]::new($false).GetBytes($Document)
            $stream.Write($bytes, 0, $bytes.Length)
        } finally {
            $stream.Dispose()
        }
    } catch {
        throw ('Windows configuration fixture creation failed (hresult=0x{0:X8}).' -f $_.Exception.HResult)
    } finally {
        $identity.Dispose()
    }
}
