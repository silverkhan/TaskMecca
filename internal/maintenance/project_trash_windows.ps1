$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
$inputRecord = [Console]::In.ReadToEnd() | ConvertFrom-Json
Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Runtime.InteropServices;

[ComImport, Guid("43826D1E-E718-42EE-BC55-A1E261C37BFE"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IShellItem {
  void BindToHandler(IntPtr pbc, ref Guid bhid, ref Guid riid, out IntPtr ppv);
  void GetParent(out IShellItem parent);
  void GetDisplayName(uint sigdn, out IntPtr name);
  void GetAttributes(uint mask, out uint attributes);
  void Compare(IShellItem other, uint hint, out int order);
}
[ComImport, Guid("947AAB5F-0A5C-4C13-B4D6-4BF7836FC9F8"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IFileOperation {
  void Advise(IFileOperationProgressSink sink, out uint cookie);
  void Unadvise(uint cookie);
  void SetOperationFlags(uint flags);
  void SetProgressMessage([MarshalAs(UnmanagedType.LPWStr)] string message);
  void SetProgressDialog(IntPtr dialog);
  void SetProperties(IntPtr properties);
  void SetOwnerWindow(IntPtr owner);
  void ApplyPropertiesToItem(IShellItem item);
  void ApplyPropertiesToItems(IntPtr items);
  void RenameItem(IShellItem item, [MarshalAs(UnmanagedType.LPWStr)] string name, IFileOperationProgressSink sink);
  void RenameItems(IntPtr items, [MarshalAs(UnmanagedType.LPWStr)] string name);
  void MoveItem(IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name, IFileOperationProgressSink sink);
  void MoveItems(IntPtr items, IShellItem destination);
  void CopyItem(IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name, IFileOperationProgressSink sink);
  void CopyItems(IntPtr items, IShellItem destination);
  void DeleteItem(IShellItem item, IFileOperationProgressSink sink);
  void DeleteItems(IntPtr items);
  void NewItem(IShellItem destination, uint attributes, [MarshalAs(UnmanagedType.LPWStr)] string name, [MarshalAs(UnmanagedType.LPWStr)] string template, IFileOperationProgressSink sink);
  void PerformOperations();
  void GetAnyOperationsAborted([MarshalAs(UnmanagedType.Bool)] out bool aborted);
}
[ComVisible(true), Guid("04B0F1A7-9490-44BC-96E1-4296A31252E2"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IFileOperationProgressSink {
  [PreserveSig] int StartOperations();
  [PreserveSig] int FinishOperations(int result);
  [PreserveSig] int PreRenameItem(uint flags, IShellItem item, [MarshalAs(UnmanagedType.LPWStr)] string name);
  [PreserveSig] int PostRenameItem(uint flags, IShellItem item, [MarshalAs(UnmanagedType.LPWStr)] string name, int result, IShellItem created);
  [PreserveSig] int PreMoveItem(uint flags, IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name);
  [PreserveSig] int PostMoveItem(uint flags, IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name, int result, IShellItem created);
  [PreserveSig] int PreCopyItem(uint flags, IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name);
  [PreserveSig] int PostCopyItem(uint flags, IShellItem item, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name, int result, IShellItem created);
  [PreserveSig] int PreDeleteItem(uint flags, IShellItem item);
  [PreserveSig] int PostDeleteItem(uint flags, IShellItem item, int result, IShellItem created);
  [PreserveSig] int PreNewItem(uint flags, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name);
  [PreserveSig] int PostNewItem(uint flags, IShellItem destination, [MarshalAs(UnmanagedType.LPWStr)] string name, [MarshalAs(UnmanagedType.LPWStr)] string template, uint attributes, int result, IShellItem created);
  [PreserveSig] int UpdateProgress(uint total, uint complete);
  [PreserveSig] int ResetTimer();
  [PreserveSig] int PauseTimer();
  [PreserveSig] int ResumeTimer();
}
[ComVisible(true), ClassInterface(ClassInterfaceType.None)]
public class RecycleSink : IFileOperationProgressSink {
  public string Source, Location;
  public uint Volume, High, Low;
  public bool Success;
  public int Result = unchecked((int)0x80004005);
  public int StartOperations() { return 0; }
  public int FinishOperations(int r) { return r; }
  public int PreRenameItem(uint f, IShellItem i, string n) { return 0; }
  public int PostRenameItem(uint f, IShellItem i, string n, int r, IShellItem c) { return 0; }
  public int PreMoveItem(uint f, IShellItem i, IShellItem d, string n) { return 0; }
  public int PostMoveItem(uint f, IShellItem i, IShellItem d, string n, int r, IShellItem c) { return 0; }
  public int PreCopyItem(uint f, IShellItem i, IShellItem d, string n) { return 0; }
  public int PostCopyItem(uint f, IShellItem i, IShellItem d, string n, int r, IShellItem c) { return 0; }
  public int PreDeleteItem(uint f, IShellItem item) {
    // Reject a Shell transfer that is not marked for recycling. In particular,
    // insufficient space/disabled Recycle Bin must never become permanent delete.
    if ((f & 0x80) == 0) return unchecked((int)0x80004004);
    try { NativeRecycle.CheckIdentity(Source, Volume, High, Low); return 0; }
    catch { return unchecked((int)0x80004004); }
  }
  public int PostDeleteItem(uint f, IShellItem item, int result, IShellItem created) {
    Result = result;
    Success = result >= 0;
    if (created != null) { try { Location = NativeRecycle.ItemPath(created); } catch {} }
    return 0;
  }
  public int PreNewItem(uint f, IShellItem d, string n) { return 0; }
  public int PostNewItem(uint f, IShellItem d, string n, string t, uint a, int r, IShellItem c) { return 0; }
  public int UpdateProgress(uint t, uint c) { return 0; }
  public int ResetTimer() { return 0; }
  public int PauseTimer() { return 0; }
  public int ResumeTimer() { return 0; }
}
public static class NativeRecycle {
  [StructLayout(LayoutKind.Sequential)] struct FileInfo {
    public uint Attributes;
    public System.Runtime.InteropServices.ComTypes.FILETIME Created, Accessed, Written;
    public uint Volume, SizeHigh, SizeLow, Links, High, Low;
  }
  [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] static extern IntPtr CreateFile(string name, uint access, uint share, IntPtr security, uint disposition, uint flags, IntPtr template);
  [DllImport("kernel32.dll", SetLastError=true)] static extern bool GetFileInformationByHandle(IntPtr file, out FileInfo info);
  [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr file);
  [DllImport("shell32.dll", CharSet=CharSet.Unicode, PreserveSig=false)] static extern void SHCreateItemFromParsingName(string path, IntPtr context, ref Guid iid, [MarshalAs(UnmanagedType.Interface)] out IShellItem item);
  public static void CheckIdentity(string path, uint volume, uint high, uint low) {
    var handle = CreateFile(path, 0x80, 7, IntPtr.Zero, 3, 0x02200000, IntPtr.Zero);
    if (handle == new IntPtr(-1)) throw new IOException("Folder inaccessible or locked");
    try {
      FileInfo info;
      if (!GetFileInformationByHandle(handle, out info) || (info.Attributes & 0x400) != 0 || info.Volume != volume || info.High != high || info.Low != low)
        throw new IOException("Folder identity or reparse point changed");
    } finally { CloseHandle(handle); }
  }
  internal static string ItemPath(IShellItem item) {
    IntPtr name; item.GetDisplayName(0x80058000, out name);
    try { return Marshal.PtrToStringUni(name); } finally { Marshal.FreeCoTaskMem(name); }
  }
  public static string Recycle(string path, uint volume, uint high, uint low) {
    CheckIdentity(path, volume, high, low);
    var operation = (IFileOperation)Activator.CreateInstance(Type.GetTypeFromCLSID(new Guid("3AD05575-8857-4850-9277-11B85BDB8E09")));
    IShellItem item = null;
    try {
      Guid iid = new Guid("43826D1E-E718-42EE-BC55-A1E261C37BFE");
      SHCreateItemFromParsingName(path, IntPtr.Zero, ref iid, out item);
      // RECYCLEONDELETE makes recycling mandatory, unlike ALLOWUNDO's fallback.
      // EARLYFAILURE prevents continuation after an error; no UI/elevation/copy hook.
      operation.SetOperationFlags(0x00080000 | 0x00100000 | 0x00800000 | 0x0004 | 0x0010 | 0x0400);
      var sink = new RecycleSink { Source=path, Volume=volume, High=high, Low=low };
      operation.DeleteItem(item, sink);
      operation.PerformOperations();
      bool aborted; operation.GetAnyOperationsAborted(out aborted);
      if (aborted || !sink.Success || Directory.Exists(path)) throw new IOException("Recycle canceled/incomplete; HRESULT 0x" + sink.Result.ToString("X8"));
      return String.IsNullOrEmpty(sink.Location) ? "Windows system Recycle Bin" : sink.Location;
    } finally {
      if (item != null) Marshal.ReleaseComObject(item);
      Marshal.ReleaseComObject(operation);
    }
  }
}
'@
$location = [NativeRecycle]::Recycle([string]$inputRecord.path, [uint32]$inputRecord.volume, [uint32]$inputRecord.high, [uint32]$inputRecord.low)
@{success=$true; location=$location} | ConvertTo-Json -Compress
