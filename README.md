catlib
======

A command line tool to concatenate static libraries into single `.lib` file (for Windows only). This tool is intended to be used as a workaround in case you get `LNK1189: the limit of 65535 objects or members in a library has been exceeded.` when trying same job with `lib` command.

requirements
============
* go
* Visual Studio

install
=======
```
go get github.com/kbinani/catlib/cmd/catlib
go install github.com/kbinani/catlib/cmd/catlib
```

usage
=====

```
Usage of catlib:
  --base string
    	file path of base static library
  --delete-default-lib
    	delete '-defaultlib:"libfoo"' from '.drectve' section when libfoo.lib is in '--input' (default true)
  --extra-lib-flags string
    	extra 'lib' command options for final concatenation stage
  --input string
    	comma separated list of file path of import libs
  --output string
    	file path of output library
Example:
  catlib --base=myproject.lib ^
         --input=zlibstat.lib,libprotobuf.lib ^
         --output=myproject-prelinked.lib ^
         --delete-default-lib ^
         --extra-lib-flags="/LTCG /WX"
```

license
=======
MIT