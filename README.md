# BOIS

Bois is either Barts Own Image Server, or a Basic Opinionated Image Server, 
or some other acronym. It is a server that stores and processes images via
a RESTful API. Specifically, it will do the following things:

## PUT requests

Upon a `PUT` request, it will process and store the supplied image data.
Image data must be sent directly in the request body; multipart-encoded
files are not now understood. If everything is OK, you'll receive a
redirection response sending you to the URL of the file. The file will not
be stored verbatim, but rather re-encoded as a JPEG.

## POST requests

Any URL so acquired can be the target of a `POST` request. Such requests
should contain a `format` parameter, which specifies an operation to be
performed. The following formats are understood:

* scale-*width*x*height* - directly scale an image to given dimensions
* clip-*width*x*height* - fit an image in the given dimensions, preserving
  the aspect ratio of the source file
* crop-*width*x*height*(-x*horizontal*y*vertical*) - crop an image to a
  given width and height, centering on the horizontal and vertical x and y,
  given as percentage points. These can be ommitted in which case they
  default to 50%, i.e. the center. Obviously 0 and 100 specify the top and
  bottom, left and right corners respectively.
* cut-*x*x*y*-t*top*l*left*(-s*width*x*height*) - cut an image of
  dimensions *x* by *y*, from *top* and *left*, and optionially scaling to
  *width* by *height* afterwards. *top* and *left* are interpreted
  literally (as opposed to percentage, as in crop).
  
If all is well the server will respond by redirecting you to the newly
acquired image.

## GET requests

`GET` requests return the image, as is. Directory readings are not
supported.

## DELETE requests

`DELETE` requests delete either an individual image (if it is 'derived'
from the source image) or the whole set of images (when you `DELETE` the
source). 



