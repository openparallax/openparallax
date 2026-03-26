---
name: file-management
description: Read, write, delete, move, copy, list, and search files and directories.
when_to_use: When the user wants to interact with files or directories on the filesystem.
actions:
  - read_file
  - write_file
  - delete_file
  - move_file
  - copy_file
  - create_directory
  - list_directory
  - search_files
keywords:
  - file
  - read
  - write
  - create
  - delete
  - move
  - copy
  - rename
  - folder
  - directory
  - list
  - find
emoji: "\U0001F4C1"
---

# File Management

Manage files and directories in the workspace and beyond.

## Actions

- **read_file**: Read the contents of a file. Parameters: `path`.
- **write_file**: Write content to a file (creates if missing). Parameters: `path`, `content`.
- **delete_file**: Delete a file. Parameters: `path`.
- **move_file**: Move or rename a file. Parameters: `source`, `destination`.
- **copy_file**: Copy a file. Parameters: `source`, `destination`.
- **create_directory**: Create a directory. Parameters: `path`.
- **list_directory**: List directory contents. Parameters: `path`.
- **search_files**: Search for files by pattern. Parameters: `path`, `pattern`.
