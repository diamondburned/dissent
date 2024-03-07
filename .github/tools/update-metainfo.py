#!/usr/bin/env python3

# Command arguments: release version (tag name), date (in format YEAR-MONTH-DAY), URL
# First argument is required, date and url are optional

import sys
import logging
import xml.etree.ElementTree as ET

from datetime import datetime

logging.basicConfig(level=logging.INFO)

METAINFO_PATH = "../../so.libdb.dissent.metainfo.xml"
REPOSITORY_URL = "https://github.com/diamondburned/dissent"

arg_list = sys.argv[1:]
rel_version = arg_list[0]

if len(arg_list) == 1:
    rel_date = datetime.today().strftime("%Y-%m-%d")
    rel_url = f"{REPOSITORY_URL}/releases/tag/{rel_version}"
else:
    rel_date = arg_list[1]
    rel_url = arg_list[2]

logging.info("Parsed release info:")
logging.info("Version: %s", rel_version)
logging.info("Date: %s", rel_date)
logging.info("Tag URL: %s", rel_url)

# Get element tree and 'component' root element
tree = ET.parse(METAINFO_PATH)
root = tree.getroot()

# Get 'releases' element
releases = root.find("releases")

# Create new 'release' element with 'url' subelement
new_release = ET.Element(
    "release",
    {
        'version': rel_version,
        'date': rel_date
    }
)

release_url = ET.SubElement(new_release, "url")
release_url.text = rel_url

# Insert 'release' as a first element of the 'releases' element
releases.insert(0, new_release)

# Indent 'releases' element for better readability
ET.indent(releases, level=1)

# Write edited XML to the original metainfo file
tree.write(METAINFO_PATH, encoding="utf-8", xml_declaration=True)
