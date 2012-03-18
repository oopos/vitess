# Copyright 2012, Google Inc.
# All rights reserved.

# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions are
# met:

#     * Redistributions of source code must retain the above copyright
# notice, this list of conditions and the following disclaimer.
#     * Redistributions in binary form must reproduce the above
# copyright notice, this list of conditions and the following disclaimer
# in the documentation and/or other materials provided with the
# distribution.
#     * Neither the name of Google Inc. nor the names of its
# contributors may be used to endorse or promote products derived from
# this software without specific prior written permission.

# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
# "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
# LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
# A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
# OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
# SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
# LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,           
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY           
# THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
# OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

from net import mc_bson_request
from vtdb import dbexceptions

class BaseCursor(object):
  arraysize = 1
  lastrowid = None
  rowcount = 0
  results = None
  connection = None
  description = None
  index = None

  def __init__(self, connection):
    self.connection = connection

  def close(self):
    self.connection = None
    self.results = None

  # pass kargs here in case higher level APIs need to push more data through
  # for instance, a key value for shard mapping
  def _execute(self, sql, bind_variables, **kargs):
    self.rowcount = 0
    self.results = None
    self.description = None
    self.lastrowid = None

    sql_check = sql.strip().lower()
    if sql_check == 'begin':
      self.connection.begin()
      return
    elif sql_check == 'commit':
      self.connection.commit()
      return
    elif sql_check == 'rollback':
      self.connection.rollback()
      return

    self.results, self.rowcount, self.lastrowid, self.description = self.connection._execute(sql, bind_variables, **kargs)
    self.index = 0
    return self.rowcount

  def fetchone(self):
    if self.results is None:
      raise dbexceptions.ProgrammingError('fetch called before execute')

    if self.index >= len(self.results):
      return None
    self.index += 1
    return self.results[self.index-1]

  def fetchmany(self, size=None):
    if self.results is None:
      raise dbexceptions.ProgrammingError('fetch called before execute')

    if self.index >= len(self.results):
      return []
    if size is None:
      size = self.arraysize
    res = self.results[self.index:self.index+size]
    self.index += size
    return res

  def fetchall(self):
    if self.results is None:
      raise dbexceptions.ProgrammingError('fetch called before execute')
    return self.fetchmany(len(self.results)-self.index)

  def callproc(self):
    raise dbexceptions.NotSupportedError

  def executemany(self, *pargs):
    raise dbexceptions.NotSupportedError

  def nextset(self):
    raise dbexceptions.NotSupportedError

  def setinputsizes(self, sizes):
    pass

  def setoutputsize(self, size, column=None):
    pass

  @property
  def rownumber(self):
    return self.index

  def __iter__(self):
    return self

  def next(self):
    val = self.fetchone()
    if val is None:
      raise StopIteration
    return val

# A simple cursor intended for attaching to a single tablet server.
class TabletCursor(BaseCursor):
  def execute(self, sql, bind_variables=None):
    return self._execute(sql, bind_variables)


# Standard cursor when connecting to a sharded backend.
class Cursor(BaseCursor):
  def execute(self, sql, bind_variables=None, key=None, keys=None):
    try:
      return self._execute(sql, bind_variables, key=key, keys=keys)
    except mc_bson_request.MCBSonException, e:
      if str(e) == 'unavailable':
        self.connection._load_tablets()
      raise

class KeyedCursor(BaseCursor):
  def __init__(self, connection, key=None, keys=None):
    self.key = key
    self.keys = keys
    BaseCursor.__init__(self, connection)

  def execute(self, sql, bind_variables):
    return self._execute(sql, bind_variables, key=self.key, keys=self.keys)

class BatchCursor(BaseCursor):
  def __init__(self, connection):
    self.exec_list = []
    BaseCursor.__init__(self, connection)

  def execute(self, sql, bind_variables=None, key=None, keys=None):
    self.exec_list.append(BatchQueryItem(sql, bind_variables, key, keys))

  def flush(self):
    self.rowcount = self.connection._exec_batch(self.exec_list)
    self.exec_list = []


# just used for batch items
class BatchQueryItem(object):
  def __init__(self, sql, bind_variables, key, keys):
    self.sql = sql
    self.bind_variables = bind_variables
    self.key = key
    self.keys = keys
