# File Operations Reference

## Functions

- [fileopen](fileopen.md) - open a file
- [fileread](fileread.md) - read from a file  
- [filewrite](filewrite.md) - write to a file
- [fileseek](fileseek.md) - seek to position in file
- [fileclose](fileclose.md) - close a file

## Synopsis

These functions provide basic file input/output operations. All functions operate on **FILE** handles returned by **fileopen**.

## Notes

Always check return values for errors. Close files when finished to avoid resource leaks.# File Operations API Reference

Welcome to the File Operations API documentation. This guide covers the essential functions for working with files in your applications.

## Core Functions

### [fileopen()](fileopen.md)
Opens a file for reading, writing, or both. This is the first step for any file operation.

### [fileread()](fileread.md) 
Reads data from an opened file. Requires the file to be opened in read mode.

### [filewrite()](filewrite.md)
Writes data to an opened file. Requires the file to be opened in write mode.

### [fileseek()](fileseek.md)
Changes the current position within an open file for random access.

### [fileclose()](fileclose.md)
Closes an open file and releases system resources. Always call this when finished.

---

## Quick Start Example

```c
// Open file for reading
FILE* fp = fileopen("data.txt", "r");
if (fp == NULL) return -1;

// Read some data
char buffer[256];
int bytes_read = fileread(buffer, sizeof(buffer), fp);

// Close the file
fileclose(fp);
```

## Best Practices

- **Always check return values** from fileopen() before using the file
- **Close files promptly** to free system resources
- **Use appropriate modes** (read/write/append) for your use case
- **Handle errors gracefully** - file operations can fail for many reasons

## See Also

For more advanced file operations, see:
- Memory mapping functions
- Asynchronous I/O operations  
- File system monitoring APIs