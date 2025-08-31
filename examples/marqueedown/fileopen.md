# fileopen

Open a file for reading or writing.

[Back to index](index.md)

## Synopsis

**FILE fileopen(filename, mode)**

## Description

Opens the file specified by **filename** according to **mode**. Returns a file handle for use with other file operations.

## Parameters

- **filename** - path to the file
- **mode** - access mode string

## Modes

- **r** - read only
- **w** - write only, truncate file
- **a** - write only, append to end
- **r+** - read and write
- **w+** - read and write, truncate file

## Return Value

Returns file handle on success, **NULL** on failure.

## Example

Opening a file for reading:

*FILE fp = fileopen("config.txt", "r");*

*if (fp == NULL) return ERROR;*

Always check the return value before using the file handle.

## See Also

[fileread](fileread.md), [filewrite](filewrite.md), [fileclose](fileclose.md)

---

**fileopen** - file operations manual