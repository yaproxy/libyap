# Golang Library powered by Yaproxy

## Package Proxy

Package proxy provides support for a variety of protocols to proxy network data.

### Example

* Basic Usage

```
import "github.com/yaproxy/libyap/proxy"
    
    ...
func X() {
    fixedURL := "http://username:password@proxy.site" // HTTP Proxy
    //fixedURL := "https://username:password@proxy.site" // HTTPS Proxy
    //fixedURL := "sock5://username:password@proxy.site" // sock5 Proxy
    // customize your dialer
    dialer := &net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }
    newDialer, err := proxy.FromURL(fixedURL, dialer, nil)
    conn, err := newDialer.Dial("tcp", "google.com:443")
    // use the tcp connection
    ...
}
```
