new Vue({
  el: '#app',
  data: {
    ws: null, // Our websocket
    newMsg: '', // Holds new messages to be sent to the server
    chatContent: '', // A running list of chat messages displayed on the screen
    username: null, // Our username
    joined: false // True if and username have been filled in
  },
  created: function() {
    var self = this;
    this.ws = new WebSocket('ws://' + window.location.host + '/ws');
    this.ws.addEventListener('message', function(e) {
      var msg = JSON.parse(e.data);
      self.chatContent +=
        '<li>' +
        '<div class="badge badge-pill badge-dark">' +
        msg.username +
        '</div>' +
        emojione.toImage(msg.message) + // Parse emojis
        '</li>';
      var element = document.getElementById('chat-messages');
      // Auto scroll to the bottom, timeout to stop race condition
      const isScrolledToBottom =
        element.scrollHeight - element.clientHeight <= element.scrollTop + 1;
      if (isScrolledToBottom) {
        setTimeout(function() {
          element.scrollTop = element.scrollHeight;
        }, 1);
      }
    });
  },
  methods: {
    send: function() {
      if (this.newMsg != '') {
        this.ws.send(
          JSON.stringify({
            username: this.username,
            message: $('<p>')
              .html(this.newMsg)
              .text() // Strip out html
          })
        );
        this.newMsg = ''; // Reset newMsg
      }
    },
    join: function() {
      if (!this.username) {
        Materialize.toast('You must choose a username', 2000);
        return;
      }
      this.username = $('<p>')
        .html(this.username)
        .text();
      this.joined = true;
      this.ws.send(
        JSON.stringify({
          username: 'bot',
          message: $('<p>')
            .html(`>>> ${this.username} has joined >>>`)
            .text() // Strip out html
        })
      );
    }
  }
});

