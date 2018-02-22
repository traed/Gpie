const fs = require('fs');
const exec = require('child_process').exec;

function slideshow() {
  exec('pkill fbi');
  exec('fbi -a -noverbose -norandom -T 1 -t 8 `find "/home/pi/bildspel" -iname "*.jpg"`', (err, stdout, stderr) => {
    if (err) {
      console.log(err);
      return;
    }
    console.log("Stdout: " + stdout)
    console.log("Stderr: " + stderr)
  });
}

slideshow();

fs.watch("/home/pi/bildspel", (e, fn) => {
  console.log(e);
  
  if (e != 'rename') {return;}
  
  slideshow();
});
