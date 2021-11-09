<html>
<head>
<title>EMPLOYEE DETAILS</title>
</head>
<body bgcolor="white">
<center><font size=20>GIT MCQ QUESTIONS</font></center>
<h2>1. HTML ______ Language.</h2><br>
<form>
  <input type="radio" name="choice" value="Scripting"> Scripting<br>
  <input type="radio" name="choice" value="Programming"> Programming<br>
  <input type="radio" name="choice" value="Application"> Application<br>
  <input type="radio" name="choice" value="None of These"> None of These<br>
</form>
<h2>2. How do I add a message to a commit?</h2><br>
<form>  
  <input type="radio" name="choice" value="$ git message "Fix error in xxxx"">$ git message "Fix error in xxxx"<br>
  <input type="radio" name="choice" value="$ git add "Fix error in xxxx"">$ git add "Fix error in xxxx<br>
  <input type="radio" name="choice" value="$ git commit "Fix error in xxxx"">$ git commit "Fix error in xxxx"<br>
  <input type="radio" name="choice" value="$ git commit -m "Fix error in xxxx"">$ git commit -m "Fix error in xxxx"<br>
</form>
<h2>3. Which comes first _____?</h2><br>
<form>
  <input type="radio" name="choice" value="$ git add">$ git add<br>
  <input type="radio" name="choice" value="$ git commit">$ git commit<br>
<h2>4. Which of the following commands used to undo a commit?</h2><br>
</form>
<form>
  <input type="radio" name="choice" value="$ git reset <commit>">$ git reset <commit><br>
  <input type="radio" name="choice" value="$ git revert <commit>">$ git revert <commit><br>
  <input type="radio" name="choice" value="$ git reset --hard origin/master">$ git reset --hard origin/master<br>
  <input type="radio" name="choice" value="$ git init">$ git init<br>
</form>
<br>
<center><button>Submit Answer</button></center>
<script>
var submitAnswer = function() {

  var radios = document.getElementsByName('choice');
  var val= "";
  for (var i = 0, length = radios.length; i < length; i++) {
      if (radios[i].checked) {
         val = radios[i].value; 
         break;
       }
  }
  
  if (val == "" ) {
    alert('please select choice answer');
  } else if ( val == "Scripting" ) {
    alert('Answer is correct !');
  } else {
    alert('Answer is wrong');
  }
};
</script>
</body>
</html>