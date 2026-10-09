"""Generate owned synthetic fixtures; QA only, Pillow with OpenJPEG required.
No network, database, original documents, or production image processing.
"""
from pathlib import Path
import hashlib
import io
import json
import struct
import zlib
from PIL import Image, features

ROOT = Path(__file__).resolve().parent


def stream(attrs, payload):
    return (f'<< {attrs} /Length {len(payload)} >>\nstream\n'.encode()
            + payload + b'\nendstream')


def pdf(name, payload, width, height, colorspace, bits, codec):
    content = b'q 144 0 0 144 36 36 cm /Im0 Do Q\nBT /F1 12 Tf 36 195 Td (Stage 01 probe) Tj ET\n'
    objects = [
        b'<< /Type /Catalog /Pages 2 0 R /Outlines 8 0 R >>',
        b'<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
        b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 216 216] /CropBox [0 0 216 216] /Resources << /XObject << /Im0 5 0 R >> /Font << /F1 6 0 R >> >> /Contents 4 0 R /Annots [7 0 R] >>',
        stream('', content),
        stream(f'/Type /XObject /Subtype /Image /Width {width} /Height {height} /ColorSpace /{colorspace} /BitsPerComponent {bits} /Filter /{codec}', payload),
        b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
        b'<< /Type /Annot /Subtype /Link /Rect [36 190 180 208] /Border [0 0 0] /A << /S /URI /URI (https://example.org/stage-01) >> >>',
        b'<< /Type /Outlines /First 9 0 R /Last 9 0 R /Count 1 >>',
        b'<< /Title (Probe bookmark) /Parent 8 0 R /Dest [3 0 R /Fit] >>',
    ]
    data = bytearray(b'%PDF-1.7\n%\xe2\xe3\xcf\xd3\n')
    offsets = [0]
    for index, obj in enumerate(objects, 1):
        offsets.append(len(data))
        data.extend(f'{index} 0 obj\n'.encode() + obj + b'\nendobj\n')
    xref = len(data)
    data.extend(f'xref\n0 {len(offsets)}\n0000000000 65535 f \n'.encode())
    for offset in offsets[1:]:
        data.extend(f'{offset:010d} 00000 n \n'.encode())
    data.extend(f'trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n'.encode())
    (ROOT / name).write_bytes(data)
    return dict(file=name, width=width, height=height, dpi=width // 2,
                filter=codec, bits=bits, sha256=hashlib.sha256(data).hexdigest())


def pattern(size, gray=False):
    im = Image.new('L' if gray else 'RGB', (size, size))
    if gray:
        im.putdata([(x * 255 // size + y * 255 // size) // 2
                    for y in range(size) for x in range(size)])
    else:
        im.putdata([(x * 255 // size, y * 255 // size, (x+y) * 127 // size)
                    for y in range(size) for x in range(size)])
    return im


fixtures = []
for size in (1200, 300):
    im = pattern(size)
    jpeg = io.BytesIO()
    im.save(jpeg, 'JPEG', quality=92, subsampling=0)
    fixtures.append(pdf(f'jpeg-{size//2}dpi.pdf', jpeg.getvalue(), size, size,
                        'DeviceRGB', 8, 'DCTDecode'))
    fixtures.append(pdf(f'flate-rgb-{size//2}dpi.pdf', zlib.compress(im.tobytes()), size, size,
                        'DeviceRGB', 8, 'FlateDecode'))
im = pattern(1200, gray=True)
fixtures.append(pdf('flate-gray-600dpi.pdf', zlib.compress(im.tobytes()), 1200, 1200,
                    'DeviceGray', 8, 'FlateDecode'))
mono = Image.new('1', (1200, 1200))
mono.putdata([255 if (x//24 + y//24) % 2 else 0
              for y in range(1200) for x in range(1200)])
fixtures.append(pdf('mono-flate-600dpi.pdf', zlib.compress(mono.tobytes()), 1200, 1200,
                    'DeviceGray', 1, 'FlateDecode'))
# Embedded JBIG2 page info + immediate generic MMR region, no global segments.
# MMR is generated losslessly by Pillow's TIFF codec, not a production codec.
tiff = io.BytesIO()
mono.point(lambda pixel: 255 - pixel).save(tiff, 'TIFF', compression='group4', strip_size=1024*1024)
tiff.seek(0)
ti = Image.open(tiff)
offsets, lengths = ti.tag_v2[273], ti.tag_v2[279]
# Force a single strip to avoid concatenating independently encoded strips.
if len(offsets) != 1:
    raise RuntimeError('JBIG2 fixture requires a single Group4 strip')
mmr = tiff.getvalue()[offsets[0]:offsets[0]+lengths[0]]
# TIFF MinIsBlack uses the opposite polarity from embedded JBIG2 MMR.
# Invert before encoding; decoded PDF pixels are asserted against mono Flate.
if ti.tag_v2[262] != 1:
    raise RuntimeError('Unexpected TIFF photometric interpretation')
def segment(number, kind, payload):
    return struct.pack('>IBBBI', number, kind, 0, 1, len(payload)) + payload
page = struct.pack('>IIIIBH', 1200, 1200, 600, 600, 0, 0)
region = struct.pack('>IIIIBB', 1200, 1200, 0, 0, 0, 1) + mmr
jbig2 = segment(1, 48, page) + segment(2, 38, region)
fixtures.append(pdf('jbig2-600dpi.pdf', jbig2, 1200, 1200, 'DeviceGray', 1, 'JBIG2Decode'))
if not features.check('jpg_2000'):
    raise RuntimeError('Positive JPX fixture requires Pillow OpenJPEG')
jpx = io.BytesIO()
pattern(1200).save(jpx, 'JPEG2000', irreversible=False)
fixtures.append(pdf('jpx-600dpi.pdf', jpx.getvalue(), 1200, 1200, 'DeviceRGB', 8, 'JPXDecode'))
(ROOT / 'manifest.json').write_text(json.dumps(fixtures, indent=2) + '\n')
print(f'Generated {len(fixtures)} synthetic fixtures')
