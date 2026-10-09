"""Stage 07 real-tools QA. Network-none container, owned synthetic data only.
Artifacts are persistent; service jobs are private tmpfs and always cleaned.
"""
import argparse
import concurrent.futures
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import threading
import time
from PIL import Image, ImageChops, ImageStat, ImageDraw

ROOT=Path('/src'); FIX=ROOT/'testdata/pdf'
commands=[]

def run(args, timeout=120, allowed=(0,)):
    p=subprocess.run([str(x) for x in args],capture_output=True,timeout=timeout)
    commands.append(dict(args=[str(x) for x in args],exit=p.returncode,stderr=p.stderr.decode(errors='replace')[:8192]))
    if p.returncode not in allowed:
        raise RuntimeError(f'{args[0]} exit {p.returncode}: {p.stderr[:500]!r}')
    return p.stdout

def dump(path, value):
    path.write_text(json.dumps(value,indent=2,ensure_ascii=False)+'\n')

def normalize_text(data):
    # Keep form-feeds/page order, all internal whitespace and Unicode exact.
    return '\n'.join(line.rstrip(' \t') for line in data.decode('utf8').replace('\r\n','\n').replace('\r','\n').split('\n')).rstrip('\n')

def inspect(path, out):
    out.mkdir(parents=True,exist_ok=True)
    run(['qpdf','--check',path])
    doc=json.loads(run(['qpdf','--json=2','--json-stream-data=none',path]))
    dump(out/'qpdf.json',doc)
    info=run(['pdfinfo','-box','-f','1','-l',str(len(doc['pages'])),path]).decode()
    (out/'pdfinfo.txt').write_text(info)
    text=run(['pdftotext','-layout',path,'-'])
    (out/'poppler.txt').write_bytes(text)
    fonts=run(['pdffonts',path]); (out/'fonts.txt').write_bytes(fonts)
    # Ignore metadata, page IDs and streams. Poppler geometry is independently recorded.
    geom=[x.strip() for x in info.splitlines() if re.match(r'(Pages:|Page\s+\d+ (size:|rot:|MediaBox:|CropBox:))',x)]
    run(['pdftoppm','-r','144','-png',path,out/'page'])
    renders=sorted(out.glob('page-*.png'))
    return doc,normalize_text(text),geom,renders

def pixel_metrics(a,b):
    a=Image.open(a).convert('RGB'); b=Image.open(b).convert('RGB')
    if a.size!=b.size: return dict(equal=False,mae=255,max=255,size_equal=False)
    diff=ImageChops.difference(a,b)
    return dict(equal=diff.getbbox() is None,mae=sum(ImageStat.Stat(diff).mean)/3,
                max=max(x[1] for x in diff.getextrema()),size_equal=True)

def mono_hashes(path,out):
    run(['pdfimages','-png',path,out/'image'])
    values=[]
    for p in sorted(out.glob('image-*.png')):
        im=Image.open(p)
        values.append(dict(size=list(im.size),sha256=hashlib.sha256(im.convert('L').tobytes()).hexdigest()))
    return sorted(values,key=lambda x:(x['size'],x['sha256']))

def worker(inputpath,profile,job):
    job.mkdir(mode=0o700,parents=True)
    req=dict(input_path=str(inputpath),work_dir=str(job),profile=profile,qpdf_path='qpdf',gs_path='gs',max_pages=500)
    env=dict(os.environ,TMPDIR=str(job),XDG_CACHE_HOME=str(job/'cache'),GOMEMLIMIT='384MiB')
    p=subprocess.run(['pdfworker'],input=json.dumps(req).encode(),capture_output=True,env=env,timeout=120)
    if p.returncode: raise RuntimeError(f'worker exit {p.returncode}')
    reply=json.loads(p.stdout)
    dump(job/'reply.json',reply)
    return reply

def quality(out):
    rows=[]; manifest=json.loads((FIX/'quality-manifest.json').read_text())
    for fixture in manifest:
        name=fixture['file']; source=FIX/name
        assert hashlib.sha256(source.read_bytes()).hexdigest()==fixture['sha256']
        srcdir=out/name.replace('.pdf','')/'source'
        try: original=inspect(source,srcdir)
        except Exception as e:
            for profile in ['lossless','balanced','compact']: rows.append(dict(fixture=name,profile=profile,status='Fail',error=str(e)))
            dump(out/'quality.json',rows); continue
        for profile in ['lossless','balanced','compact']:
            dest=out/name.replace('.pdf','')/profile; dest.mkdir(parents=True,exist_ok=True)
            row=dict(fixture=name,profile=profile,features=fixture['features'],status='Fail',checks={})
            try:
                job=Path('/tmp/quality-jobs')/name/profile
                reply=worker(source,profile,job)
                if fixture.get('http_status'):
                    assert reply.get('status')==fixture['http_status'],reply
                    assert not (job/'gs-candidate.pdf').exists(),'negative rewritten'
                    row.update(status='Pass',refusal=reply['code']); continue
                assert 'result' in reply,reply
                result=reply['result']; row['result']=result
                selected=dest/'selected.pdf'; shutil.copyfile(result['output_path'],selected)
                # Save rejected raw rewrite as evidence, but don't claim it passed acceptance.
                for artifact in ['gs-candidate.pdf','gs-structural.pdf','lossless.pdf']:
                    if (job/artifact).exists(): shutil.copyfile(job/artifact,dest/artifact)
                candidate=inspect(selected,dest/'selected')
                checks=row['checks']
                checks['not_larger']=selected.stat().st_size<=source.stat().st_size
                checks['unchanged_exact']=result['status']!='unchanged' or selected.read_bytes()==source.read_bytes()
                checks['poppler_text_page_order']=candidate[1]==original[1]
                checks['poppler_geometry']=candidate[2]==original[2]
                checks['page_count']=len(candidate[0]['pages'])==len(original[0]['pages'])
                metrics=[pixel_metrics(a,b) for a,b in zip(original[3],candidate[3])]
                row['render_metrics']=metrics
                exact=(profile=='lossless' or result['engine']!='ghostscript')
                # Appearance screening only: MAE <= 6/255, supplements semantic checks,
                # decoded symbols/mono equality and mandatory manual crop inspection.
                checks['render']=len(original[3])==len(candidate[3]) and all(m['equal'] if exact else m['size_equal'] and m['mae']<=fixture.get('appearance_mae_limit',6) for m in metrics)
                row['render_rule']='exact pixels' if exact else f"MAE <= {fixture.get('appearance_mae_limit',6)}/255 plus manual review"
                if fixture.get('mono'):
                    checks['mono_decoded_pixels']=mono_hashes(source,srcdir)==mono_hashes(selected,dest/'selected')
                    if (dest/'gs-candidate.pdf').exists():
                        rawdir=dest/'raw-mono'; rawdir.mkdir(exist_ok=True)
                        checks['raw_gs_mono_pixels']=mono_hashes(source,srcdir)==mono_hashes(dest/'gs-candidate.pdf',rawdir)
                if fixture.get('decode'):
                    expected=fixture['decode']
                    decoded=[]
                    for img in candidate[3]:
                        decoded.append(run(['zbarimg','--quiet',img],allowed=(0,4)).decode())
                    checks['symbol_decode']=all(expected in s for s in decoded)
                    row['decoded']=decoded
                if fixture.get('warning') and profile!='lossless':
                    checks['truthful_fallback']=fixture['warning'] in result['warnings'] and result['engine']!='ghostscript'
                if fixture.get('known_size'):
                    # Test actual raw GS rewrite too, even when smallest selects qpdf.
                    raw=dest/'gs-candidate.pdf' if profile!='lossless' else selected
                    if raw.exists():
                        images=json.loads(run(['qpdf','--json=2','--json-stream-data=none',raw]))['pages'][0]['images']
                        width=fixture['known_size']; expected=width
                        if width==1200 and not fixture.get('mono') and profile!='lossless': expected=600 if profile=='balanced' else 300
                        row['raw_image_dimensions']=[(i['width'],i['height']) for i in images]
                        checks['known_dpi_no_upscale']=any(i['width']==expected and i['height']==expected for i in images)
                row['status']='Pass' if all(checks.values()) else 'Fail'
                if row['status']=='Fail': print('FAIL',name,profile,checks,flush=True)
            except Exception as e:
                row['error']=str(e); print('ERROR',name,profile,str(e),flush=True)
            finally:
                rows.append(row)
                shutil.rmtree(Path('/tmp/quality-jobs')/name/profile,ignore_errors=True)
                dump(out/'quality.json',rows)
    # Codec decoder cross-check: CCITT and JBIG2 synthetic patterns equal mono Flate.
    hashes={name:mono_hashes(FIX/name,out/name.replace('.pdf','')/'source') for name in ['mono-flate-600dpi.pdf','ccitt-600dpi.pdf','jbig2-600dpi.pdf']}
    dump(out/'mono-codec-crosscheck.json',dict(hashes=hashes,equal=len({json.dumps(v,sort_keys=True) for v in hashes.values()})==1))
    # Contact sheets of source/returned compact; further manual crops can use these PNGs.
    for fixture in manifest:
        if fixture.get('http_status'): continue
        name=fixture['file'].replace('.pdf',''); a=sorted((out/name/'source').glob('page-*.png'))
        b=sorted((out/name/'compact'/'selected').glob('page-*.png'))
        if not b: continue
        for i,(pa,pb) in enumerate(zip(a,b)):
            sheet=Image.new('RGB',(900,470),'white'); draw=ImageDraw.Draw(sheet)
            for x,p,label in [(0,pa,'original'),(450,pb,'compact returned')]:
                im=Image.open(p).convert('RGB'); im.thumbnail((440,440)); sheet.paste(im,(x,25)); draw.text((x+5,5),name+' '+label,fill='black')
            sheet.save(out/f'{name}-comparison-{i+1}.png')
    return all(r['status']=='Pass' for r in rows) and len({json.dumps(v,sort_keys=True) for v in hashes.values()})==1

class Server:
    def __init__(self,out,workers):
        self.jobs=Path('/work/jobs/runtime'); self.jobs.mkdir(exist_ok=True)
        self.log=(out/f'server-{workers}.log').open('wb')
        env=dict(os.environ,LISTEN_ADDR='127.0.0.1:18087',TEMP_DIR=str(self.jobs),MAX_WORKERS=str(workers),WORKER_PATH='/usr/local/bin/pdfworker')
        self.p=subprocess.Popen(['server'],env=env,stdout=self.log,stderr=self.log)
        deadline=time.monotonic()+10
        while time.monotonic()<deadline:
            try:
                c=http.client.HTTPConnection('127.0.0.1',18087,timeout=1); c.request('GET','/readyz'); r=c.getresponse(); r.read(); c.close()
                if r.status==200: return
            except OSError: pass
            time.sleep(.05)
        raise RuntimeError('server not ready')
    def stop(self):
        self.p.terminate(); self.p.wait(timeout=10); self.log.close()
    def post(self,path,profile='balanced',save=None):
        boundary='stage07owned'
        head=f'--{boundary}\r\nContent-Disposition: form-data; name="profile"\r\n\r\n{profile}\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="owned.pdf"\r\nContent-Type: application/pdf\r\n\r\n'.encode()
        tail=f'\r\n--{boundary}--\r\n'.encode()
        start=time.monotonic(); c=http.client.HTTPConnection('127.0.0.1',18087,timeout=180)
        c.putrequest('POST','/v1/compress'); c.putheader('Content-Type','multipart/form-data; boundary='+boundary); c.putheader('Content-Length',str(len(head)+path.stat().st_size+len(tail))); c.endheaders()
        c.send(head)
        with path.open('rb') as f:
            while chunk:=f.read(65536): c.send(chunk)
        c.send(tail); upload=time.monotonic()-start
        r=c.getresponse(); response_ready=time.monotonic()-start-upload; status=r.status; headers=dict(r.getheaders()); size=0
        f=save.open('wb') if save else None
        while chunk:=r.read(65536):
            size+=len(chunk)
            if f: f.write(chunk)
        if f: f.close()
        c.close()
        return dict(status=status,headers=headers,output_bytes=size,seconds=time.monotonic()-start,upload_seconds=upload,processing_until_headers_seconds=response_ready)
    def clean(self):
        deadline=time.monotonic()+5
        while time.monotonic()<deadline:
            if not list(self.jobs.glob('pdfjob-*')): return True
            time.sleep(.02)
        return False

class Monitor:
    def __init__(self,jobs):
        self.jobs=jobs; self.stop_event=threading.Event(); self.peak_memory=0; self.peak_disk=0; self.samples=0; self.max_jobs=0; self.peak_anon=0; self.peak_shmem=0; self.peak_allocated=0; self.peak_single_job=0
        self.t=threading.Thread(target=self.loop,daemon=True); self.t.start()
    def loop(self):
        while not self.stop_event.is_set():
            p=Path('/sys/fs/cgroup/memory.current')
            if p.exists(): self.peak_memory=max(self.peak_memory,int(p.read_text()))
            stats=Path('/sys/fs/cgroup/memory.stat')
            if stats.exists():
                values=dict(line.split() for line in stats.read_text().splitlines()); self.peak_anon=max(self.peak_anon,int(values.get('anon',0))); self.peak_shmem=max(self.peak_shmem,int(values.get('shmem',0)))
            allocated=0; perjob={}
            total=0
            for root,dirs,files in os.walk(self.jobs):
                for name in files:
                    try:
                        st=(Path(root)/name).stat(); total+=st.st_size; allocated+=st.st_blocks*512
                        job=Path(root).relative_to(self.jobs).parts[0] if Path(root)!=self.jobs else '.'
                        perjob[job]=perjob.get(job,0)+st.st_size
                    except FileNotFoundError: pass
            self.peak_allocated=max(self.peak_allocated,allocated); self.peak_single_job=max([self.peak_single_job]+list(perjob.values())); self.peak_disk=max(self.peak_disk,total); self.max_jobs=max(self.max_jobs,len(list(self.jobs.glob('pdfjob-*')))); self.samples+=1
            self.stop_event.wait(.02)
    def done(self):
        self.stop_event.set(); self.t.join()
        return dict(sample_ms=20,samples=self.samples,peak_cgroup_memory_bytes=self.peak_memory,peak_job_logical_bytes=self.peak_disk,peak_job_allocated_bytes=self.peak_allocated,peak_single_job_bytes=self.peak_single_job,peak_anon_bytes=self.peak_anon,peak_shmem_bytes=self.peak_shmem,max_active_jobs=self.max_jobs)

def benchmark(out):
    fixtures=out/'load-fixtures'; rows=[]
    for workers in [1,2]:
        server=Server(out,workers)
        try:
            for name in ['pages-500.pdf','pages-500-50mib.pdf','dedup-unique.pdf','dedup-duplicate.pdf']:
                for profile in (['lossless','balanced','compact'] if name=='pages-500-50mib.pdf' else ['balanced']):
                    monitor=Monitor(server.jobs); start=time.monotonic()
                    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
                        futures=[pool.submit(server.post,fixtures/name,profile,out/f'load-{workers}-{name}-{profile}-{i}.pdf') for i in range(workers)]
                        replies=[f.result() for f in futures]
                    measurement=monitor.done(); cleanup=server.clean()
                    row=dict(workers=workers,fixture=name,profile=profile,total_seconds=time.monotonic()-start,replies=replies,cleanup=cleanup,**measurement)
                    row['status']='Pass' if cleanup and measurement['max_active_jobs']==workers and all(r['status']==200 and r['output_bytes']<=(fixtures/name).stat().st_size for r in replies) else 'Fail'
                    # Verify max-load page count/order/geometry structurally plus Poppler text.
                    for i,r in enumerate(replies):
                        if r['status']==200:
                            p=out/f'load-{workers}-{name}-{profile}-{i}.pdf'; run(['qpdf','--check',p])
                            if name.startswith('pages-500'):
                                before=normalize_text(run(['pdftotext','-layout',fixtures/name,'-'])); after=normalize_text(run(['pdftotext','-layout',p,'-']))
                                assert before==after,'load page order/text changed'
                    rows.append(row); dump(out/'benchmark.json',rows); print('LOAD',workers,name,profile,row['status'],round(row['total_seconds'],2),flush=True)
            # Actual page and bytes limits, busy before upload, cancellation under load.
            r=server.post(fixtures/'pages-501.pdf','lossless')
            rows.append(dict(case='501 pages',workers=workers,status='Pass' if r['status']==422 and server.clean() else 'Fail',reply=r))
            # Add one byte to exact boundary via a local synthetic copy; upload streaming.
            oversized=Path('/tmp/over-limit.pdf'); shutil.copyfile(fixtures/'pages-500-50mib.pdf',oversized)
            with oversized.open('ab') as f: f.write(b'\n')
            r=server.post(oversized,'lossless'); oversized.unlink()
            rows.append(dict(case='50MiB+1',workers=workers,status='Pass' if r['status']==413 and server.clean() else 'Fail',reply=r))
        finally: server.stop()
    # Dedup microbenchmark actual adapter arguments; alternate on/off 3 times.
    dedup=[]
    for name in ['dedup-duplicate.pdf','dedup-unique.pdf']:
        for trial in range(3):
            for enabled in ([True,False] if trial%2==0 else [False,True]):
                output=Path('/tmp/dedup-output.pdf')
                args=json.loads(run(['quality-profiles','balanced',fixtures/name,output]))
                args=[a.replace('-dDetectDuplicateImages=true','-dDetectDuplicateImages='+str(enabled).lower()) for a in args]
                before=time.monotonic(); run(['gs']+args); elapsed=time.monotonic()-before
                dedup.append(dict(fixture=name,trial=trial,dedup=enabled,seconds=elapsed,bytes=output.stat().st_size,args=args)); output.unlink()
    dump(out/'dedup.json',dedup); dump(out/'benchmark.json',rows)
    return all(r['status']=='Pass' for r in rows)

def main():
    p=argparse.ArgumentParser(); p.add_argument('--mode',choices=['quality','load'],required=True); p.add_argument('--out',default='/work/jobs'); args=p.parse_args()
    out=Path(args.out)
    if args.mode=='quality' and (out/'quality.json').exists():
        raise SystemExit('Choose a fresh --out directory; existing acceptance evidence is preserved')
    out.mkdir(exist_ok=True,parents=True)
    versions={tool:run([tool,flag],allowed=(0,)).decode(errors='replace') for tool,flag in [('gs','--version'),('qpdf','--version'),('pdftoppm','-v'),('zbarimg','--version')]}
    # Poppler version normally appears on stderr; preserve the actual captured output.
    for tool in versions:
        if not versions[tool].strip():
            versions[tool]=next(c['stderr'] for c in commands if c['args'][0]==tool)
    dump(out/f'{args.mode}-versions.json',versions)
    try: passed=quality(out) if args.mode=='quality' else benchmark(out)
    finally: dump(out/f'{args.mode}-commands.json',commands)
    print(args.mode,'Pass' if passed else 'Fail',flush=True)
    raise SystemExit(0 if passed else 1)

if __name__=='__main__': main()
