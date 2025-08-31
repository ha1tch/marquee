# File Operations

Basic file I/O functions for C programs.

## Functions

- [fileopen](fileopen.md) - open a file
- [fileread](fileread.md) - read data from file
- [filewrite](filewrite.md) - write data to file
- [fileseek](fileseek.md) - move position in file
- [fileclose](fileclose.md) - close file

## Usage

All file operations start with **fileopen** and end with **fileclose**.

Example workflow:

1. Open file with [fileopen](fileopen.md)
2. Read or write data
3. Close file with [fileclose](fileclose.md)

---

Always check return values for errors. Close files when finished to avoid resource leaks.