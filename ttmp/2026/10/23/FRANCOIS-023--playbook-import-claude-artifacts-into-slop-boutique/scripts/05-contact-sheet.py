import sys, glob
from PIL import Image
fs=sorted(sys.argv[2:]); ims=[Image.open(f).convert('RGB') for f in fs]
W=480; th=[im.resize((W,int(im.height*W/im.width))) for im in ims]
cols=4; rows=(len(th)+cols-1)//cols; rh=[max(t.height for t in th[r*cols:(r+1)*cols]) for r in range(rows)]
sheet=Image.new('RGB',(W*cols,sum(rh)),'white'); y=0
for r in range(rows):
    for c,t in enumerate(th[r*cols:(r+1)*cols]): sheet.paste(t,(c*W,y))
    y+=rh[r]
sheet.save(sys.argv[1]); print(fs)
