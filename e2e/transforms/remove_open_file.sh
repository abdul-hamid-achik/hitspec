#!/bin/sh
# Simulates another program deleting the file the studio has open (the
# studio_deleted_on_disk spec). glyph's transform step needs an output file, so
# write one.
rm -f /tmp/hitspec-studio-e2e/deleted/c.http
echo done > "$GLYPHRUN_OUTPUT"
