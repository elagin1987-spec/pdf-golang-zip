"""Owned synthetic PDFs for nested-action policy; stdlib only, no external data."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def pdf(catalog="", page="", link="", extra=""):
    objects = [
        f"<< /Type /Catalog /Pages 2 0 R {catalog} >>".encode(),
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        f"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 216 216] /Resources << >> /Contents 4 0 R {page} >>".encode(),
        b"<< /Length 0 >>\nstream\n\nendstream",
    ]
    if link:
        objects.append(f"<< /Type /Annot /Subtype /Link /Rect [10 10 100 30] {link} >>".encode())
    if extra:
        objects.append(extra.encode())
    data = bytearray(b"%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
    offsets = [0]
    for n, obj in enumerate(objects, 1):
        offsets.append(len(data))
        data.extend(f"{n} 0 obj\n".encode() + obj + b"\nendobj\n")
    xref = len(data)
    data.extend(f"xref\n0 {len(offsets)}\n0000000000 65535 f \n".encode())
    for offset in offsets[1:]:
        data.extend(f"{offset:010d} 00000 n \n".encode())
    data.extend(f"trailer\n<< /Root 1 0 R /Size {len(offsets)} >>\nstartxref\n{xref}\n%%EOF\n".encode())
    return bytes(data)


js = "<< /S /JavaScript /JS (app.alert\\(1\\)) >>"
uri = "/A << /S /URI /URI (https://example.org) >>"
cases = [
    ("catalog-aa", dict(catalog=f"/AA << /WC {js} >>"), 422),
    ("page-aa", dict(page=f"/AA << /O {js} >>"), 422),
    ("link-aa", dict(page="/Annots [5 0 R]", link=f"{uri} /AA << /E {js} >>"), 422),
    ("next-dictionary", dict(catalog=f"/OpenAction << /S /GoTo /D [3 0 R /Fit] /Next {js} >>"), 422),
    ("next-array-launch", dict(catalog="/OpenAction << /S /GoTo /D [3 0 R /Fit] /Next [<< /S /Launch /F (owned.txt) >>] >>"), 422),
    ("indirect-aa", dict(page="/Annots [5 0 R]", link=f"{uri} /AA << /E 6 0 R >>", extra=js), 422),
    ("clean-uri", dict(page="/Annots [5 0 R]", link=uri), 200),
    ("clean-goto", dict(page="/Annots [5 0 R]", link="/A << /S /GoTo /D [3 0 R /Fit] >>"), 200),
]
manifest = []
for name, args, status in cases:
    data = pdf(**args)
    filename = name + ".pdf"
    (ROOT / filename).write_bytes(data)
    manifest.append(dict(file=filename, sha256=hashlib.sha256(data).hexdigest(), http_status=status,
                         code="special_type_javascript" if status == 422 else ""))
(ROOT / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
