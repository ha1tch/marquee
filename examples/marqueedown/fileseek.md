# fileseek

Change the current position within an open file.

[Back to index](index.md)

## Synopsis

**int** fileseek(**file**, **offset**, **whence**)

## Description

Sets the file position indicator for **file** to **offset** bytes from the location specified by **whence**. This allows random access to file contents.

## Parameters

- **file** - file handle from [fileopen](fileopen.md)
- **offset** - number of bytes to move
- **whence** - reference point for the offset

## Whence Values

- **SEEK_SET** - offset from beginning of file
- **SEEK_CUR** - offset from current position  
- **SEEK_END** - offset from end of file

## Return Value

Returns **0** on success, **-1** on error.

## Examples

Seek to beginning of file:

*fileseek(fp, 0, SEEK_SET);*

Seek to end of file:

*fileseek(fp, 0, SEEK_END);*

Move 100 bytes forward from current position:

*fileseek(fp, 100, SEEK_CUR);*

Seek to 50 bytes before end:

*fileseek(fp, -50, SEEK_END);*

## Notes

- Position is measured in bytes from the reference point
- Seeking beyond end of file may extend the file on some systems
- Text files may not support all seek operations reliably

## Errors

Returns **-1** if:

- File is not open
- Invalid whence value
- Seek operation not supported for this file type
- I/O error occurs

## See Also

[fileopen](fileopen.md), [fileread](fileread.md), [filewrite](filewrite.md)

---

**fileseek** - file operations manual