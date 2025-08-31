# fileclose

Close an open file and release resources.

[Back to index](index.md)

## Synopsis

**int** fileclose(**file**)

## Description

Closes the file associated with **file** and releases all system resources. Any buffered data is written to the file before closing.

## Parameters

- **file** - file handle from [fileopen](fileopen.md)

## Return Value

Returns **0** on success, **-1** on error.

## Important Notes

- **Always close files** when finished to prevent resource leaks
- Buffered data is automatically flushed before closing
- Once closed, the file handle becomes invalid
- Closing a **NULL** handle is safe (no operation)

## Example

Basic file closing:

*FILE fp = fileopen("data.txt", "r");*

*// ... use the file ...*

*if (fileclose(fp) != 0) {*
*    printf("Warning: Error closing file\n");*
*}*

## Resource Management

Files consume system resources. Each open file uses:

- Memory for buffers
- File descriptors (limited per process)
- Locks that may block other programs

Closing files promptly is essential for:

- **Performance** - prevents resource exhaustion
- **Reliability** - ensures data is saved
- **Compatibility** - allows other programs to access files

## Errors

Returns **-1** if:

- I/O error occurs during final flush
- File system error
- Invalid file handle

Even if **fileclose** returns an error, the file handle becomes invalid and should not be used again.

## Best Practice

Use this pattern for robust file handling:

*FILE fp = fileopen("file.txt", "r");*
*if (fp == NULL) return ERROR;*

*// ... file operations ...*

*int result = fileclose(fp);*
*if (result != 0) handle_close_error();*

## See Also

[fileopen](fileopen.md), [fileread](fileread.md), [filewrite](filewrite.md), [fileseek](fileseek.md)

---

**fileclose** - file operations manual