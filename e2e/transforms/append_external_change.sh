#!/bin/sh
# Simulates another program editing the file the studio has open (the
# studio_save_conflict spec): append a line to it. glyph's transform step needs
# an output file, so write one.
echo '# changed elsewhere' >> /tmp/hitspec-studio-e2e/conflict/c.http
echo done > "$GLYPHRUN_OUTPUT"
