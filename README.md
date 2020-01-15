# forganizer

forganizer is an utility that organizes files into foldres according to file modification date.
Can skip files newer than defined amount of days. If a target file with the same content exists it removes the source file.
In case files are different it renames source file by adding _X suffix.

## Use case
You have many photos in your phone camera folder.
You run 
```
forganizer -r -d 30 /phone/camera /desktop/photoarchive/
```

and it moves all files into /Year/Month directory structure
leaving let's say 30 last days of photos on your phone.
