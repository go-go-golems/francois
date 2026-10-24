import json, sys
from PIL import Image
for s in sys.argv[1:]:
    g=json.load(open(f'{s}/gallery.json'))[s]
    W,H=400,250; cols=5; rows=(len(g)+cols-1)//cols
    sh=Image.new('RGB',(W*cols,H*rows),'white')
    for i,(f,a) in enumerate(g):
        im=Image.open(f).convert('RGB'); im.thumbnail((W-6,H-6)); sh.paste(im,((i%cols)*W,(i//cols)*H))
    sh.save(f'review-{s}.png')
