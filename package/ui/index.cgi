#!/bin/sh
# DSM runs this CGI for /webman/3rdparty/vtunerradio/index.cgi/...
# vtunerd checks the caller's DSM login and forwards the request to the local server.
exec /var/packages/vtunerradio/target/bin/vtunerd cgi -data /var/packages/vtunerradio/var
