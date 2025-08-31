# fileopen

Open a file for reading or writing.

[Back to index](index.md)

## Synopsis

**FILE** fileopen(**filename**, **mode**)

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

## See Also

[fileread](fileread.md), [filewrite](filewrite.md), [fileclose](fileclose.md)

---

**fileopen** - file operations manual# fileopen()

Opens a file for reading, writing, or both operations.

← [Back to API Index](index.md)

## Syntax

```c
FILE* fileopen(const char* filename, const char* mode);
```

## Parameters

- **filename**: Path to the file to open
- **mode**: Access mode string specifying how to open the file

## Return Value

Returns a `FILE*` pointer on success, or `NULL` if the operation fails.

## File Modes

| Mode | Description | File Position |
|------|-------------|---------------|
| `"r"` | Read only | Beginning |
| `"w"` | Write only (truncates existing) | Beginning |
| `"a"` | Write only (append) | End |
| `"r+"` | Read and write | Beginning |
| `"w+"` | Read and write (truncates existing) | Beginning |
| `"a+"` | Read and write (append) | End |

## Error Handling

`fileopen()` returns `NULL` when it fails. Common failure reasons:

- **File not found** (when opening for read)
- **Permission denied** (insufficient access rights)
- **Path does not exist** (directory doesn't exist)
- **Too many open files** (system limit reached)
- **Invalid filename** (illegal characters or format)

Always check the return value before using the file pointer.

## Examples

### Basic File Opening

```c
// Open file for reading
FILE* input = fileopen("config.txt", "r");
if (input == NULL) {
    printf("Error: Cannot open config.txt for reading\n");
    return -1;
}

// Use the file...
// Remember to close when done
fileclose(input);
```

### Opening Multiple Files

```c
FILE* input = fileopen("source.dat", "r");
FILE* output = fileopen("destination.dat", "w");

if (input == NULL || output == NULL) {
    printf("Error opening files\n");
    if (input) fileclose(input);
    if (output) fileclose(output);
    return -1;
}

// Process data from input to output...

fileclose(input);
fileclose(output);
```

### Append Mode Example

```c
// Open log file for appending
FILE* log = fileopen("application.log", "a");
if (log == NULL) {
    printf("Warning: Cannot open log file\n");
    return;
}

// Write log entry
filewrite("Application started\n", 19, log);
fileclose(log);
```

## Best Practices

1. **Always check for NULL** before using the returned file pointer
2. **Use appropriate modes** - don't open for write if you only need to read
3. **Close files promptly** using [fileclose()](fileclose.md) to free resources
4. **Handle errors gracefully** - file operations can fail in production environments

## Related Functions

- [fileread()](fileread.md) - Read data from the opened file
- [filewrite()](filewrite.md) - Write data to the opened file  
- [fileseek()](fileseek.md) - Change position within the file
- [fileclose()](fileclose.md) - Close the file when finished

---

**Next**: Learn how to [read data](fileread.md) from your opened file.