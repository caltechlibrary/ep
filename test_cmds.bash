#!/bin/bash

#
# Run some command line tests to confirm working cli
#

if [[ -d "testout" ]]; then
    rm -fR testout
fi
mkdir testout

#
# Jira Issue DR-40
#
echo 'Testing DR-40'
./bin/doi2eprintxml -i testdata/DR-40-test.txt > testout/dr40-eprint.xml
T=$(grep '<date>1942-07</date>' testout/dr40-eprint.xml)
if [[ "${T}" != "" ]]; then
    echo "expected '<data>1942-07</date>', got '${T}'"
    exit 1
fi

#
# Jira Iusse DR-43
#
echo 'Testing DR-43'
./bin/doi2eprintxml -i testdata/DR-43-test.txt > testout/dr43-eprint.xml
T=$(grep '<date>2001-01</date>' testout/dr43-eprint.xml)
if [[ "${T}" != "" ]]; then
    echo "expected '<data>2001-01</date>', got '${T}'"
    exit 1
fi




#
# Jira Issue DR-45
#
echo 'Testing DR-45 (this takes a while)'
./bin/doi2eprintxml -i testdata/DR-45-test.txt > testout/dr45-eprint.xml
if [[ "$?" != "0" ]]; then
    echo ''
    echo 'Testing doi2eprintxml DR-45 issue failed.'
    exit 1
fi

#
# Jira Issue DR-59
#
echo 'Testing DR-59'
./bin/doi2eprintxml -i testdata/DR-59-test.txt > testout/dr-59-eprint.xml
if [[ "$?" != "0" ]]; then
    echo ''
    echo 'Testing doi2eprintxml DR-45 issue failed.'
    exit 1
fi

#
# Jira Issue DR-141
#
echo 'Testing DR-141'
./bin/doi2eprintxml -i testdata/DR-141-test.txt > testout/dr-141-eprint.xml
if [[ "$?" != "0" ]]; then
    echo ''
    echo 'Testing doi2eprintxml DR-141 issue failed.'
    exit 1
fi


echo 'OK, passed cli tests'
