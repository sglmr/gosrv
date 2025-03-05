# GoSrv - Simple Go Development Server with Live Reload

GoSrv is a lightweight, zero-dependency development server for static web projects. It serves your HTML, CSS, and JavaScript files with automatic browser refresh when changes are detected.

## Features

- **Zero Dependencies**: Built entirely with Go's standard library
- **Live Reload**: Automatically refreshes your browser when files change
- **Simple Setup**: Just one file, no configuration needed
- **Cross-Platform**: Works on Windows, macOS, and Linux

## Installation

1. Ensure you have Go installed (version 1.16 or later recommended)
2. Download the `main.go` file or clone this repository
3. Run the server with `go run main.go`

## Usage

```bash
# Basic usage (serves current directory on port 8080)
go run main.go

# Specify a different port
go run main.go -addr=:3000

# Specify a different directory to serve
go run main.go -dir=./demo

# Specify both
go run main.go -addr=:3000 -dir=./demo
```

## How It Works

GoSrv does the following:

1. **Static File Serving**: Serves your static files like HTML, CSS, and JavaScript
2. **Directory Monitoring**: Watches your files for changes every 500ms
3. **Live Reload Injection**: Injects a small JavaScript snippet into HTML files
4. **Server-Sent Events**: Uses SSE to notify the browser when to reload

When you make changes to any file in the served directory, all connected browsers will automatically refresh.

## Customization

- Change the polling interval by modifying the `watchDirectoryForChanges` call
- Exclude more directories by updating the filtering in `scanDirectory`
- Modify the reload behavior by changing the injected JavaScript

## Limitations

- Uses polling rather than file system events, which is less efficient but more portable
- No HTTPS support (for local development only)
- No build process integration (serves files as-is)

## Use Cases

Ideal for:
- Simple static website development
- Testing HTML/CSS/JS before deployment
- Learning web development with minimal tooling
- Quick prototyping without complex build tools

## License

MIT License - Feel free to use, modify, and distribute as needed!

## Contributing

Contributions welcome! Feel free to open issues or submit pull requests.