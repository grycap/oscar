#!/bin/bash

  basedir=$1

  # Add double // to the basedir path
  parsedbasedir=${basedir//\//\\\/}

  # Debug
  echo "BASEDIR: $basedir"
  echo "PARSED BASEDIR: $parsedbasedir"

  # Sed new value on AppDir on project file
  sed -i "s/<AppDir>/<AppDir>'${parsedbasedir}'/g" ../project.xml

  # Exit with last command value
  exit

