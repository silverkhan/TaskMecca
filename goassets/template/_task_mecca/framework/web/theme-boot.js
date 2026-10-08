// Apply the saved appearance before style.css and the main application load.
// This blocking, same-origin script is allowed by the existing CSP.
(() => {
  const root=document.documentElement;
  let storedTheme='',storedPalette='';
  try {
    storedTheme=localStorage.getItem('task-mecca-theme')||'';
    storedPalette=localStorage.getItem('task-mecca-palette')||'';
  }catch(_){}
  const preference=['dark','light','system'].includes(storedTheme)?storedTheme:'dark';
  const palette=['mecca','slate'].includes(storedPalette)?storedPalette:'mecca';
  const systemDark=window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ?? false;
  const effective=preference==='system'?(systemDark?'dark':'light'):preference;
  const background=palette==='mecca'
    ?(effective==='dark'?'#0a0e1b':'#f7f3f8')
    :(effective==='dark'?'#111318':'#f7f8fa');
  root.dataset.themePreference=preference;
  root.dataset.theme=effective;
  root.dataset.palette=palette;
  // Cover the time before the external stylesheet becomes available.
  root.style.colorScheme=effective;
  root.style.backgroundColor=background;
  const meta=document.querySelector('meta[name="theme-color"]');
  if(meta)meta.setAttribute('content',background);
})();
