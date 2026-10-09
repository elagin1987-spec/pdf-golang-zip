"""Owned Stage 07 synthetic fixtures. QA only; no network/DB/private documents.
Uses bundled Pillow/reportlab/pypdf; existing Stage 01 fixtures are not regenerated.
"""
import hashlib
import io
import json
import random
import struct
import zlib
from pathlib import Path
from PIL import Image
from reportlab.pdfgen import canvas
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.graphics.barcode import qrencoder, code128
from pypdf import PdfReader, PdfWriter
from pypdf.generic import NameObject, NumberObject, FloatObject

ROOT = Path(__file__).resolve().parent
LOAD = ROOT.parent.parent / 'artifacts/stage-07/load-fixtures'
rows = []

def stream(attrs, payload):
    return f'<< {attrs} /Length {len(payload)} >>\nstream\n'.encode() + payload + b'\nendstream'

class PDF:
    def __init__(self):
        self.objects = [b'', b'']
        self.pages = []
    def add(self, obj):
        self.objects.append(obj.encode() if isinstance(obj, str) else obj)
        return len(self.objects)
    def page(self, content, resources='', extras=''):
        c = self.add(stream('', content))
        p = self.add(f'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 216 216] /CropBox [0 0 216 216] /Resources << {resources} >> /Contents {c} 0 R {extras} >>')
        self.pages.append(p)
        return p
    def save(self, path, catalog=''):
        self.objects[0] = f'<< /Type /Catalog /Pages 2 0 R {catalog} >>'.encode()
        self.objects[1] = f'<< /Type /Pages /Kids [{" ".join(f"{p} 0 R" for p in self.pages)}] /Count {len(self.pages)} >>'.encode()
        data = bytearray(b'%PDF-1.7\n%\xe2\xe3\xcf\xd3\n')
        offsets = [0]
        for i, obj in enumerate(self.objects, 1):
            offsets.append(len(data))
            data.extend(f'{i} 0 obj\n'.encode() + obj + b'\nendobj\n')
        start = len(data)
        data.extend(f'xref\n0 {len(offsets)}\n0000000000 65535 f \n'.encode())
        for n in offsets[1:]:
            data.extend(f'{n:010d} 00000 n \n'.encode())
        data.extend(f'trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{start}\n%%EOF\n'.encode())
        path.write_bytes(data)


def record(name, features, **kw):
    path = ROOT/name
    rows.append(dict(file=name, features=features, sha256=hashlib.sha256(path.read_bytes()).hexdigest(), **kw))


def image_pdf(name, data, attrs, features, extras='', **kw):
    d = PDF()
    im = d.add(stream('/Type /XObject /Subtype /Image '+attrs, data))
    font = d.add('<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>')
    d.page(b'q 144 0 0 144 36 36 cm /Im Do Q\nBT /F 10 Tf 12 200 Td (Quality owned fixture) Tj ET', f'/XObject << /Im {im} 0 R >> /Font << /F {font} 0 R >>', extras)
    d.save(ROOT/name)
    record(name, features, **kw)

# Baseline checksums, real codecs and known placements.
for r in json.loads((ROOT/'manifest.json').read_text()):
    record(r['file'], ['baseline', r['filter']], mono=r['bits']==1,
           known_size=r['width'], target_size=(600 if r['width']==1200 and r['bits']==8 else r['width']))

size = 1200
indexed = bytes((x//32+y//32)%4 for y in range(size) for x in range(size))
image_pdf('indexed-600dpi.pdf', zlib.compress(indexed), '/Width 1200 /Height 1200 /BitsPerComponent 8 /ColorSpace [/Indexed /DeviceRGB 3 <ffffff000000e0202080b0ff>] /Filter /FlateDecode', ['Indexed','FlateDecode'],appearance_mae_limit=8)
gray = bytes((x+y)%256 for y in range(300) for x in range(300))
pred = b''.join(b'\x00'+gray[y*300:(y+1)*300] for y in range(300))
image_pdf('predictor-gray.pdf', zlib.compress(pred), '/Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceGray /Filter /FlateDecode /DecodeParms << /Predictor 15 /Colors 1 /BitsPerComponent 8 /Columns 300 >>', ['PNG-predictor','FlateDecode'])
# Constant-width 9-bit LZW with frequent Clear before dictionary growth (EarlyChange=1).
codes=[]
for start in range(0,len(gray),200):
    codes.extend([256]+list(gray[start:start+200]))
codes.append(257)
bits=''.join(f'{c:09b}' for c in codes)
bits+='0'*((-len(bits))%8)
lzw=int(bits,2).to_bytes(len(bits)//8,'big')
image_pdf('lzw-gray.pdf', lzw, '/Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceGray /Filter /LZWDecode', ['LZWDecode'])
rle=b''.join(bytes([len(gray[i:i+128])-1])+gray[i:i+128] for i in range(0,len(gray),128))+b'\x80'
image_pdf('runlength-gray.pdf', rle, '/Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceGray /Filter /RunLengthDecode', ['RunLengthDecode'])
mono = Image.new('1',(1200,1200))
mono.putdata([255 if (x//24+y//24)%2 else 0 for y in range(1200) for x in range(1200)])
t = io.BytesIO()
mono.save(t,'TIFF',compression='group4',strip_size=1024*1024)
ti=Image.open(io.BytesIO(t.getvalue()))
assert len(ti.tag_v2[273])==1
payload=t.getvalue()[ti.tag_v2[273][0]:ti.tag_v2[273][0]+ti.tag_v2[279][0]]
image_pdf('ccitt-600dpi.pdf',payload,'/Width 1200 /Height 1200 /BitsPerComponent 1 /ColorSpace /DeviceGray /Filter /CCITTFaxDecode /DecodeParms << /K -1 /Columns 1200 /Rows 1200 /BlackIs1 true >>',['CCITTFaxDecode'],mono=True)

# Form nesting + one shared high-resolution image placed at 600/1200 dpi, rotated and mirrored.
d=PDF()
rgb=Image.new('RGB',(1200,1200)); rgb.putdata([(x*255//1200,y*255//1200,(x+y)*127//1200) for y in range(1200) for x in range(1200)])
im=d.add(stream('/Type /XObject /Subtype /Image /Width 1200 /Height 1200 /BitsPerComponent 8 /ColorSpace /DeviceRGB /Filter /FlateDecode',zlib.compress(rgb.tobytes())))
f=d.add(stream(f'/Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << /XObject << /Im {im} 0 R >> >>',b'/Im Do'))
f2=d.add(stream(f'/Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << /XObject << /Inner {f} 0 R >> >>',b'/Inner Do'))
d.page(b'q 144 0 0 144 12 12 cm /Im Do Q q 72 0 0 72 140 140 cm /Nested Do Q q 0 36 -36 0 212 20 cm /Im Do Q q -36 0 0 36 212 70 cm /Im Do Q',f'/XObject << /Im {im} 0 R /Nested {f2} 0 R >>')
d.save(ROOT/'forms-shared.pdf'); record('forms-shared.pdf',['nested-Form','shared','rotation','mirror'])
# SMask and explicit 1-bit mask on a smooth RGB image; transparency over vector background.
for masktype in ['SMask','Mask']:
    d=PDF()
    n=300
    mask=Image.new('L' if masktype=='SMask' else '1',(n,n))
    mask.putdata([x*255//n if masktype=='SMask' else (255 if (x//25+y//25)%2 else 0) for y in range(n) for x in range(n)])
    attr='/ColorSpace /DeviceGray /BitsPerComponent 8' if masktype=='SMask' else '/ImageMask true /BitsPerComponent 1'
    m=d.add(stream(f'/Type /XObject /Subtype /Image /Width {n} /Height {n} {attr} /Filter /FlateDecode',zlib.compress(mask.tobytes())))
    im=d.add(stream(f'/Type /XObject /Subtype /Image /Width {n} /Height {n} /ColorSpace /DeviceRGB /BitsPerComponent 8 /{masktype} {m} 0 R /Filter /FlateDecode',zlib.compress(rgb.resize((n,n)).tobytes())))
    d.page(b'0.9 0.7 0 rg 0 0 216 216 re f q 144 0 0 144 36 36 cm /Im Do Q',f'/XObject << /Im {im} 0 R >>')
    name=masktype.lower()+'.pdf'; d.save(ROOT/name); record(name,[masktype,'transparency'])
d=PDF(); inline=b'q 144 0 0 144 36 36 cm BI /W 2 /H 2 /CS /RGB /BPC 8 /F /AHx ID ff000000ff000000ffffffff> EI Q'
d.page(inline); d.save(ROOT/'inline.pdf'); record('inline.pdf',['inline'])

# Embedded DejaVu Sans with Cyrillic, unembedded Helvetica, hidden OCR, vector diagram,
# small 6pt labels, true QR and Code128, URI/GoTo links and ordered bookmarks.
fontpath=Path('/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf')
pdfmetrics.registerFont(TTFont('OwnedDejaVu',str(fontpath)))
qr=qrencoder.QRCode(version=2,errorCorrectLevel=qrencoder.QRErrorCorrectLevel.M)
qr.addData('STAGE07-QUALITY-2026'); qr.make()
modules=qr.modules
symbol=Image.new('1',(len(modules)+8,len(modules)+8),1)
for y,line in enumerate(modules):
    for x,black in enumerate(line):
        symbol.putpixel((x+4,y+4),0 if black else 1)
symbol=symbol.resize((1200,1200),Image.Resampling.NEAREST)
image_pdf('qr-mono.pdf',zlib.compress(symbol.tobytes()),'/Width 1200 /Height 1200 /BitsPerComponent 1 /ColorSpace /DeviceGray /Filter /FlateDecode',['QR','mono'],mono=True,decode='QR-Code:STAGE07-QUALITY-2026')
c=canvas.Canvas(str(ROOT/'text-ocr-vectors.pdf'),pagesize=(432,432),pageCompression=0,invariant=1)
for page in range(1,4):
    c.setFont('OwnedDejaVu',14); c.drawString(20,400,f'Сторінка {page}: якість PDF, ґєії')
    c.setFont('Helvetica',8); c.drawString(20,380,f'PAGE {page} unembedded font - small text')
    c.bookmarkPage(f'p{page}'); c.addOutlineEntry(f'Page {page}',f'p{page}')
    c.linkURL('https://example.org/quality', (20,375,300,390),relative=0,thickness=0)
    c.linkRect('', 'p1', (20,355,300,370),relative=0,thickness=0)
    c.setFont('Helvetica',6); c.drawString(20,360,'Go to first page; 6pt label 0123456789')
    c.setStrokeColorRGB(0.1,0.3,0.7); c.setLineWidth(0.5)
    for k in range(8):
        c.line(20,310-k*10,180,310-k*10); c.circle(200+k*15,270,6,stroke=1,fill=0)
    barcode=code128.Code128('STAGE07-128',barWidth=1.0,barHeight=48,humanReadable=True)
    barcode.drawOn(c,20,170)
    # A visible scan plus independently extractable invisible OCR text; no OCR performed.
    c.drawInlineImage(rgb.resize((300,300)),20,20,120,120)
    text=c.beginText(20,40); text.setFont('OwnedDejaVu',10); text.setTextRenderMode(3); text.textLine(f'OCR шар {page}'); c.drawText(text)
    c.showPage()
c.save(); record('text-ocr-vectors.pdf',['Cyrillic','embedded-font','unembedded-font','OCR','vectors','small-text','Code128','URI','GoTo','bookmarks','page-order'],decode='CODE-128:STAGE07-128')
# Physical geometry with UserUnit, Rotate and nontrivial CropBox via pypdf.
r=PdfReader(str(ROOT/'forms-shared.pdf')); w=PdfWriter(); p=r.pages[0]
p[NameObject('/UserUnit')]=FloatObject(2); p[NameObject('/Rotate')]=NumberObject(90)
p.cropbox.lower_left=(6,6); p.cropbox.upper_right=(210,210)
w.add_page(p)
with (ROOT/'geometry.pdf').open('wb') as out: w.write(out)
record('geometry.pdf',['UserUnit','Rotate','CropBox','shared'])
# Complex-color fallback both CMYK and ICCBased (owned generated sRGB profile).
image_pdf('cmyk.pdf',zlib.compress(bytes([0,90,180,10])*300*300),'/Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceCMYK /Filter /FlateDecode',['CMYK'],warning='complex_color_preserved')
from PIL import ImageCms
d=PDF(); icc=d.add(stream('/N 3',ImageCms.ImageCmsProfile(ImageCms.createProfile('sRGB')).tobytes()))
im=d.add(stream(f'/Type /XObject /Subtype /Image /Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace [/ICCBased {icc} 0 R] /Filter /FlateDecode',zlib.compress(rgb.resize((300,300)).tobytes())))
d.page(b'q 144 0 0 144 36 36 cm /Im Do Q',f'/XObject << /Im {im} 0 R >>'); d.save(ROOT/'icc.pdf'); record('icc.pdf',['ICCBased'],warning='complex_color_preserved')
# Valid structural negative fixtures; no active JavaScript execution by QA.
for name,cat in [('special-acroform.pdf','/AcroForm << /Fields [<< /FT /Tx /T (owned) /V (synthetic) >>] >>'),('special-tagged.pdf','/StructTreeRoot << /Type /StructTreeRoot /K [] >>'),('special-javascript.pdf','/OpenAction << /S /JavaScript /JS (void 0) >>')]:
    d=PDF()
    if name=='special-acroform.pdf':
        field=d.add('<< /FT /Tx /T (owned) /V (synthetic) >>'); cat=f'/AcroForm << /Fields [{field} 0 R] >>'
    d.page(b''); d.save(ROOT/name,cat); record(name,['negative'],http_status=422)
# Remaining special-type and supported-decoder/policy-fallback boundaries.
for name,cat in [
    ('special-xfa.pdf','/AcroForm << /XFA (synthetic) >>'),
    ('special-signature.pdf','/Perms << /DocMDP << /Type /Sig /ByteRange [0 0 0 0] >> >>'),
    ('special-attachments.pdf','/Names << /EmbeddedFiles << /Names [(owned) << /Type /Filespec /F (owned.txt) >>] >> >>'),
    ('special-layers.pdf','/OCProperties << /OCGs [] /D << >> >>'),
    ('special-annotation.pdf','')]:
    d=PDF()
    extras=''
    if 'annotation' in name:
        annot=d.add('<< /Type /Annot /Subtype /Text /Rect [10 10 20 20] /Contents (owned) >>'); extras=f'/Annots [{annot} 0 R]'
    d.page(b'',extras=extras); d.save(ROOT/name,cat); record(name,['negative'],http_status=422)
d=PDF()
xmp=b'<?xpacket begin=""?><x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/" pdfaid:part="1" pdfaid:conformance="B"/></rdf:RDF></x:xmpmeta><?xpacket end="w"?>'
metadata=d.add(stream('/Type /Metadata /Subtype /XML',xmp)); d.page(b''); d.save(ROOT/'special-pdfa.pdf',f'/Metadata {metadata} 0 R'); record('special-pdfa.pdf',['negative'],http_status=422)
image_pdf('asciihex-fallback.pdf',gray.hex().encode()+b'>','/Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceGray /Filter /ASCIIHexDecode',['unsupported-filter-policy'],warning='unsupported_image_filter')
(ROOT/'quality-manifest.json').write_text(json.dumps(rows,indent=2,ensure_ascii=False)+'\n')

# Load fixtures are generated artifacts, not added 50MiB source fixtures.
LOAD.mkdir(parents=True,exist_ok=True)
for count in [500,501]:
    d=PDF(); font=d.add('<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>')
    for i in range(count):
        d.page(f'BT /F 12 Tf 20 100 Td (PAGE {i+1:04d}) Tj ET'.encode(),f'/Font << /F {font} 0 R >>')
    d.save(LOAD/f'pages-{count}.pdf')
d=PDF(); rng=random.Random(7); font=d.add('<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>')
for i in range(500):
    im=d.add(stream('/Type /XObject /Subtype /Image /Width 186 /Height 186 /BitsPerComponent 8 /ColorSpace /DeviceRGB',rng.randbytes(186*186*3)))
    d.page(f'q 144 0 0 144 36 36 cm /Im Do Q BT /F 8 Tf 12 200 Td (PAGE {i+1:04d}) Tj ET'.encode(),f'/XObject << /Im {im} 0 R >> /Font << /F {font} 0 R >>')
d.save(LOAD/'pages-500-50mib.pdf')
p=LOAD/'pages-500-50mib.pdf'; remaining=(50<<20)-p.stat().st_size
assert remaining>=2
data=p.read_bytes(); xref=data.rfind(b'\nxref\n')+1
assert xref>0
tail=data[xref:].replace(f'startxref\n{xref}\n'.encode(),f'startxref\n{xref+remaining}\n'.encode())
p.write_bytes(data[:xref]+b'%'+b'P'*(remaining-2)+b'\n'+tail)
assert p.stat().st_size==50<<20
for duplicate in [True,False]:
    d=PDF()
    for page in range(8):
        resources=[]; contents=[]
        for j in range(8):
            seed=7 if duplicate else page*8+j
            payload=random.Random(seed).randbytes(300*300*3)
            im=d.add(stream('/Type /XObject /Subtype /Image /Width 300 /Height 300 /BitsPerComponent 8 /ColorSpace /DeviceRGB /Filter /FlateDecode',zlib.compress(payload)))
            resources.append(f'/Im{j} {im} 0 R'); contents.append(f'q 50 0 0 50 {8+(j%4)*52} {30+(j//4)*80} cm /Im{j} Do Q')
        d.page('\n'.join(contents).encode(),'/XObject << '+' '.join(resources)+' >>')
    d.save(LOAD/('dedup-duplicate.pdf' if duplicate else 'dedup-unique.pdf'))
loadrows=[dict(file=p.name,bytes=p.stat().st_size,sha256=hashlib.sha256(p.read_bytes()).hexdigest()) for p in sorted(LOAD.glob('*.pdf'))]
(LOAD/'manifest.json').write_text(json.dumps(loadrows,indent=2)+'\n')
print(f'{len(rows)} quality fixtures; {len(loadrows)} load fixtures')
