# Hub

A simple HTTP server utility for quickly sharing files and text between devices on your local network.

## Build

```bash
./build.sh
```

This builds the binary and moves it to `~/bin/hub`.
Add `~/bin` to your `PATH` if you want to use it from anywhere.

## Usage

Hub runs an HTTP server on port 8003 and displays your local IP address.

### Serve a file

```bash
hub /path/to/file.txt
```

Visiting `http://<your-ip>:8003` will download the file.
Note, there are some file types like html where it is just served directly as-is.
This makes it easy to serve web pages quickly.

### Serve a directory

```bash
hub /path/to/directory
```

The directory will be zipped and served as a download.

### Serve text

```bash
hub "some text to share"
```

Or pipe from stdin:

```bash
echo "hello world" | hub
```

The text will be displayed when you visit the URL.

### Accept file and text uploads

```bash
hub upload
```

Visit `http://<your-ip>:8003` to see an upload form.
Files will be saved to an `upload/` directory.
Text will be printed to stdout.

## Example

```bash
$ hub myfile.pdf
Serving myfile.pdf
Serving on 192.168.1.100:8003
```

Then from another device, visit `http://192.168.1.100:8003` to download the file.
