# thumbnails

This project creates thumbnails of jpg files replicating the dir structure of the original (source) directory tree.

It also runs as a web server returning full size images ans thumbnail images on demand.

## Usage (not a Server)

``` bash
thumbnails source-path dest-path size=200 mask=%YYYY_%MM_%DD_%h_%m_%s_%n.%x noclobber=true
```

| Value | Desc | Optional |
| ----------- | ----------- | ----------- |
| source-path | is the root directory containing the original pictures (.jpg or .png) | required|
| dest-path | is the root directory that will contain the thumbnail pictures (.jpg) | required|
| size=N | is the minimum width or height for the thumbnail depending on the aspect ratio | optional = 200 |
| mask=M | is the format of the file name of the thumbnail created | optional = See below |
| noclobber=T | if 'true' then existing thumbnails will not be overwritten | optional = false |
| verbose | if present then event data is logged | optional = not verbose |
| serverport=P | if present runs as a server on that port | optional = not a server |
| help | will display the help text | optional = false |

The dest-path is assumed to be empty. All required directories will be created.

If the picture is taller than wide the thumbnail will be \<size\> wide.
If the picture is wider than tall the thumbnail will be \<size\> high.

The thumbnails will be oriented according to the exif --> orientation field in the jpg meta data.

If the exif --> orientation cannot be derived then the it is assumed to be 1 (rotate 0 degrees)

## Mask

The default mask is '%YYYY_%MM_%DD_%h_%m_%s_%n.%x'

| Value | Desc |
| ----------- | ----------- |
| %YYYY | is a 4 digit year |
| %MM | is a 2 digit month  |
| %DD | is a 2 digit day of month |
| %h | is a 2 digit hour in 24 hour format |
| %m | is a 2 digit minute |
| %s | is a 2 digit second |
| %n | is the name of the original file without the suffix (.jpg) |
| %x | if always 'jpg' which is the format of the thumbnail file. |

The time used is derived from the meta data in the original image.

If that is not available then the file name is parsed for a time.

If that fails then the file system 'modified' time is used.

As a last resort the current date time is used.

## Usage as a Server

``` bash
thumbnails source-path serverport=port verbose
```

Run as a server loading image files from source-path. The verbose option is optional and defaults to quiet.

``` http
http://<ipaddress>:<port>/image/full/a/b/image.jpg
```

Will load the full image from source-path/a/b/image.jpg

The following image types can be returned:

| File Extension | Content-Type |
| ----------- | ----------- |
| ".jpg" |   "image/jpeg" |
| ".jpeg" |  "image/jpeg" |
| ".gif" |   "image/gif" |
| ".bmp" |   "image/bmp" |
| ".png" |   "image/png" |
| ".tiff" |  "image/tiff" |
| ".tif" |   "image/tiff" |
| ".svg" |   "image/svg+xml" |
| ".ico" |   "image/vnd.microsoft.icon" |

``` http
http://<ipaddress>:<port>/image/200/a/b/image.jpg
```

Note that the size (200 above) must be more than 10

Will load the image from source-path/a/b/image.jpg and return it's thumbnail with a size of 200.

The following image types can be returned as thumbnails:

| File Extension | Content-Type |
| ----------- | ----------- |
| ".jpg" |   "image/jpeg" |
| ".jpeg" |  "image/jpeg" |
| ".png" |   "image/png" |

### Stopping the server

``` http
http://<ipaddress>:<port>/control/close
```

The server will close after 2 seconds.

## Thanks

ref: https://github.com/rwcarlsen/goexif (rwcarlsen) for the excelelent EXIF library.

ref: https://pkg.go.dev/github.com/liujiawm/graphics-go (liujiawm) for porting the graphics library from the original Google code.
