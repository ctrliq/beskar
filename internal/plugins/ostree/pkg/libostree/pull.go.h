#include <stdlib.h>
#include <glib.h>
#include <glib-object.h>
#include <gio/gio.h>

// The following is a mechanism for converting a Go slice of strings to a char**.  This could have been done in Go, but
// it's easier and less error prone to do it here.
char** MakeRefArray(int size) {
    return calloc(sizeof(char*), size);
}

void AppendRef(char** refs, int index, char* ref) {
    refs[index] = ref;
}

void FreeRefArray(char** refs) {
    int i;
    for (i = 0; refs[i] != NULL; i++) {
        free(refs[i]);
    }
    free(refs);
}

// This exists because cGo doesn't provide a way to cast char** to const char *const *.
void g_variant_builder_add_refs(GVariantBuilder *builder, char** refs) {
    g_variant_builder_add(
        builder,
        "{s@v}",
        "refs",
        g_variant_new_variant(
            g_variant_new_strv((const char *const *) refs, -1)
        )
    );
}