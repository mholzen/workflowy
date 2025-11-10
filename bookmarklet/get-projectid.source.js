(function() {
  var selection = window.getSelection();
  if (!selection.rangeCount) {
    alert('No selection found. Please select some text first.');
    return;
  }

  var node = selection.anchorNode;

  // If the node is a text node, start from its parent element
  if (node.nodeType === Node.TEXT_NODE) {
    node = node.parentElement;
  }

  // Walk up the DOM tree to find an element with projectid attribute
  while (node && node !== document.body) {
    if (node.getAttribute && node.getAttribute('projectid')) {
      var projectId = node.getAttribute('projectid');

      // Copy to clipboard
      navigator.clipboard.writeText(projectId).then(function() {
        alert('Project ID copied to clipboard: ' + projectId);
      }).catch(function() {
        // Fallback if clipboard API fails
        prompt('Project ID (Ctrl+C to copy):', projectId);
      });
      return;
    }
    node = node.parentElement;
  }

  alert('No element with projectid attribute found in the parent chain.');
})();
