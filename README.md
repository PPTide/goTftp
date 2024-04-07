# goTFTP
Very simple TFTP server written in Go. 
If there are any problems, please open an issue. 
This was hacked together in one evening, so it's not perfect.

## Usage
```
goTFTP [-h] [-p port] [-d directory]
```

## Options
```
-h: Show this help message and exit
-p port: Port to listen on (default: 69)
-d directory: Directory to serve files from (default: current directory)
```

## Example
```
goTFTP -p 6969 -d /path/to/directory
```