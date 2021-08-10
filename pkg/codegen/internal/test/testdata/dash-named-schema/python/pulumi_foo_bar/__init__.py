# coding=utf-8
# *** WARNING: this file was generated by test. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

from . import _utilities
import typing
# Export this package's modules as members:
from .provider import *
from ._inputs import *
from . import outputs

# Make subpackages available:
if typing.TYPE_CHECKING:
    import pulumi_foo_bar.submodule1 as __submodule1
    submodule1 = __submodule1
else:
    submodule1 = _utilities.lazy_import('pulumi_foo_bar.submodule1')

_utilities.register(
    resource_modules="""
[
 {
  "pkg": "foo-bar",
  "mod": "submodule1",
  "fqn": "pulumi_foo_bar.submodule1",
  "classes": {
   "foo-bar:submodule1:ModuleResource": "ModuleResource"
  }
 }
]
""",
    resource_packages="""
[
 {
  "pkg": "foo-bar",
  "token": "pulumi:providers:foo-bar",
  "fqn": "pulumi_foo_bar",
  "class": "Provider"
 }
]
"""
)
