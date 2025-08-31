# fileread

Read data from an open file.

[Back to index](index.md)

## Synopsis

**int** fileread(**buffer**, **size**, **file**)

## Description

Reads up to **size** bytes from **file** into **buffer**. The file must have been opened with [fileopen](fileopen.md) in a mode that allows reading.

## Parameters

- **buffer** - destination for read data
- **size** - maximum bytes to read
- **file** - file handle from fileopen

## Return Value

Returns number of bytes actually read, or **-1** on error.

## Notes

The function reads up to **size** bytes but may return fewer if:

- End of file is reached
- An error occurs during reading
- Fewer bytes are available

Always check the return value to determine how many bytes were actually read.

## Example

Reading from a file:

*char data[256];*

*int bytes = fileread(data, 256, fp);*

*if (bytes < 0) handle_error();*

## Errors

Returns **-1** if:

- File is not open for reading
- I/O error occurs
- Invalid parameters

## See Also

[fileopen](fileopen.md), [filewrite](filewrite.md), [fileseek](fileseek.md)

---

**fileread** - file operations manual