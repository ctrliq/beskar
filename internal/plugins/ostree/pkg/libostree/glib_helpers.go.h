#include <stdlib.h>
#include <glib.h>
#include <glib-object.h>
#include <gio/gio.h>

static char *
_g_error_get_message (GError *error)
{
  g_assert (error != NULL);
  return error->message;
}