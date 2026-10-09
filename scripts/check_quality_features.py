"""Independent pypdf semantic check of saved Stage 07 results; local files only.
No production snapshot code, object IDs or stream equality assumptions.
"""
import argparse
import json
from pathlib import Path
from pypdf import PdfReader
from pypdf.generic import IndirectObject


def snapshot(path):
    reader=PdfReader(path)
    refs={page.indirect_reference.idnum:i+1 for i,page in enumerate(reader.pages)}
    def destination(dest):
        if isinstance(dest,str):
            dest=reader.named_destinations[dest]
        if hasattr(dest,'dest_array'): dest=dest.dest_array
        elif isinstance(dest,IndirectObject): dest=dest.get_object()
        if isinstance(dest,dict): dest=dest['/D']
        target=dest[0]
        page=refs[target.idnum] if isinstance(target,IndirectObject) else int(target)+1
        return [page]+[None if str(v)=='NullObject' else str(v) for v in dest[1:]]
    def outlines(nodes):
        result=[]
        for node in nodes:
            if isinstance(node,list): result.append(outlines(node))
            else: result.append(dict(title=str(node['/Title']),dest=destination(node)))
        return result
    pages=[]
    for page in reader.pages:
        links=[]
        unit=float(page.get('/UserUnit',1))
        for ref in page.get('/Annots',[]):
            annot=ref.get_object()
            if annot.get('/Subtype')!='/Link': continue
            rect=[round(float(v)*unit,4) for v in annot['/Rect']]
            action=annot.get('/A')
            if action:
                action=action.get_object()
                target=['URI',str(action['/URI'])] if action['/S']=='/URI' else ['GoTo',destination(action['/D'])]
            else: target=['GoTo',destination(annot['/Dest'])]
            links.append(dict(rect=rect,target=target))
        pages.append(dict(media=[round(float(v)*unit,4) for v in page.mediabox],crop=[round(float(v)*unit,4) for v in page.cropbox],rotate=int(page.get('/Rotate',0))%360,links=sorted(links,key=lambda v:json.dumps(v,sort_keys=True))))
    return dict(pages=pages,outlines=outlines(reader.outline))


def compare(a,b):
    # Compare physical coordinates within the contract's 0.01 point tolerance.
    if type(a)!=type(b): return False
    if isinstance(a,float): return abs(a-b)<=.01
    if isinstance(a,dict): return a.keys()==b.keys() and all(compare(a[k],b[k]) for k in a)
    if isinstance(a,list): return len(a)==len(b) and all(compare(x,y) for x,y in zip(a,b))
    return a==b


def main():
    parser=argparse.ArgumentParser();parser.add_argument('artifacts',type=Path);args=parser.parse_args()
    root=Path(__file__).resolve().parents[1]
    rows=json.loads((args.artifacts/'quality.json').read_text());results=[]
    for row in rows:
        if 'result' not in row: continue
        name=row['fixture']; before=snapshot(root/'testdata/pdf'/name)
        after=snapshot(args.artifacts/name.replace('.pdf','')/row['profile']/'selected.pdf')
        results.append(dict(fixture=name,profile=row['profile'],equal=compare(before,after),original=before,returned=after))
    (args.artifacts/'independent-features.json').write_text(json.dumps(results,indent=2,ensure_ascii=False)+'\n')
    print(len(results),'independent physical geometry/URI/GoTo/bookmark checks;',sum(not r['equal'] for r in results),'failures')
    raise SystemExit(1 if any(not r['equal'] for r in results) else 0)

if __name__=='__main__':main()
