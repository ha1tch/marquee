# filewrite

Write data to an open file.

[Back to index](index.md)

## Synopsis

**int** filewrite(**data**, **size**, **file**)

## Description

Writes **size** bytes from **data** to **file**. The file must have been opened with [fileopen](fileopen.md) in a mode that allows writing.

## Parameters

- **data** - source data to write
- **size** - number of bytes to write
- **file** - file handle from fileopen

## Return Value

Returns number of bytes actually written, or **-1** on error.

## Notes

The function attempts to write **size** bytes but may write fewer if:

- Disk space runs out
- An I/O error occurs
- The file system has write restrictions

Always check that the return value equals **size** to ensure all data was written.

## Example

Writing to a file:

*char message[] = "Hello World";*

*int written = filewrite(message, 11, fp);*

*if (written != 11) handle_error();*

## Buffering

Data may be buffered by the system. To ensure data is written immediately, close the file or use system-specific flush operations.

## Errors

Returns **-1** if:

- File is not open for writing
- Disk full or quota exceeded
- I/O error occurs
- Invalid parameters

## See Also

[fileopen](fileopen.md), [fileread](fileread.md), [fileclose](fileclose.md)

---

**filewrite** - file operations manual