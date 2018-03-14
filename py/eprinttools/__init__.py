#
# eprinttools package is a Python 3 wrapper around the eprinttools Go package compiled to a C-shared library.
# 
# @author R. S. Doiel, <rsdoiel@library.caltech.edu>
#
# Copyright (c) 2018, Caltech
# All rights not granted herein are expressly reserved by Caltech.
# 
# Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
# 
# 1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
# 
# 2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
# 
# 3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
# 
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
# 
import ctypes
import os
import json


# Figure out shared library extension
go_basename = 'libeprinttools'
uname = os.uname().sysname
ext = '.so'
if uname == 'Darwin':
    ext = '.dylib'
if uname == 'Windows':
    ext = '.dll'

# Find our shared library and load it
dir_path = os.path.dirname(os.path.realpath(__file__))
lib = ctypes.cdll.LoadLibrary(os.path.join(dir_path, go_basename+ext))

# Setup our Go functions to be nicely wrapped
go_version = lib.version
go_version.restype = ctypes.c_char_p

go_is_verbose = lib.is_verbose
go_is_verbose.restype = ctypes.c_int

go_verbose_on = lib.verbose_on
go_verbose_on.restype = ctypes.c_int

go_verbose_off = lib.verbose_off
go_verbose_off.restype = ctypes.c_int

go_get_keys = lib.get_keys
go_get_keys.argtypes = [ctypes.c_char_p]
go_get_keys.restype = ctypes.c_char_p

go_get_metadata = lib.get_metadata
go_get_metadata.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int]
go_get_metadata.restype = ctypes.c_char_p

#
# Now write our Python idiomatic function
#

# is_verbose returns true is verbose is enabled, false otherwise
def is_verbose():
    ok = go_is_verbose()
    return (ok == 1)

# verbose_on turns verboseness off
def verbose_on():
    ok = go_verbose_on()
    return (ok == 1)

# verbose_off turns verboseness on
def verbose_off():
    ok = go_verbose_off()
    return (ok == 1)

# Returns version of eprinttools shared library
def version():
    value = go_version()
    if not isinstance(value, bytes):
        value = value.encode('utf-8')
    return value.decode() 

def readcfg(fname = "config.json"):
    with open(fname, mode = "r", encoding = "utf-8") as f:
        src = f.read()
        return json.loads(src)
    return {}

def get_keys(cfg):
    c = json.dumps(cfg).encode("utf-8")
    value = go_get_keys(ctypes.c_char_p(c))
    if not isinstance(value, bytes):
        value = value.encode("utf-8")
    rval = value.decode() 
    if rval == "":
        return []
    return json.loads(rval)

def get_metadata(cfg, key, save = False):
    c = json.dumps(cfg).encode("utf-8")
    k = key.encode("utf-8")
    i_save = 0
    if save == True:
        i_save = 1 
    value = go_get_metadata(ctypes.c_char_p(c), ctypes.c_char_p(k), ctypes.c_int(i_save))
    if not isinstance(value, bytes):
        value = value.encode("utf-8")
    rval = value.decode() 
    if rval == "":
        return {}
    return json.loads(rval)
