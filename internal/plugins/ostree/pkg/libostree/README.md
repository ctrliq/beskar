# ostree

ostree is a wrapper around [libostree](https://github.com/ostreedev/ostree) that aims to provide an idiomatic API.


### Notes
1. A minimal glib implementation exists within the ostree pkg. This is to avoid a dependency on glib for the time being.
   - This implementation is not complete and will be expanded as needed.
   - The glib implementation is not intended to be used outside of the ostree pkg.
   - `GCancellable` is not implemented. Just send nil.
2. Not all of libostree is wrapped. Only the parts that are needed for beskar are wrapped. Which is basically everything 
    need to perform pull operations.
    - `OstreeAsyncProgress` is not implemented. Just send nil.
      

### Developer Warnings
- `glib/gobject` are used here and add a layer of complexity to the code, specifically with regard to memory management.
glib/gobject are reference counted and objects are freed when the reference count reaches 0. Therefore, you will see 
`C.g_XXX_ref_sink` or `C.g_XXX_ref` (increases reference count) and `C.g_XXX_unref()` (decrease reference count) in some
places and `C.free()` in others.  A good rule of thumb is that if you see a `g_` prefix you are dealing with a reference
counted object and should not call `C.free()`.  See [glib](https://docs.gtk.org/glib/index.html) for more information. 
See [gobject](https://docs.gtk.org/gobject/index.html) for more information.
