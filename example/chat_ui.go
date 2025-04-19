package main

import "github.com/labstack/echo/v4"

func chatUI(ctx echo.Context) error {
	return ctx.HTML(200, `
<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <title>Hairkatz AI Chat</title>
  <style>
    body {
      margin: 0;
      font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
      background: #111315;
      display: flex;
      justify-content: center;
      align-items: center;
      height: 100vh;
    }
    .chat-container, .name-container {
      width: 100%;
      max-width: 480px;
      background: #292C2D;
      border-radius: 12px;
      box-shadow: 0 8px 16px rgba(0,0,0,0.3);
      padding: 24px;
      border: 1px solid #333;
      display: flex;
      flex-direction: column;
      gap: 16px;
    }
    .name-container {
      color: white;
    }
    .name-container input {
      padding: 12px;
      font-size: 16px;
      border: none;
      border-radius: 8px;
      outline: none;
      background: #111315;
      color: white;
    }
    .name-container button {
      background: #c39858;
      color: white;
      border: none;
      padding: 12px;
      font-size: 16px;
      cursor: pointer;
      border-radius: 8px;
      font-weight: bold;
    }
    .chat-header {
      background: #262626;
      color: white;
      padding: 16px;
      font-size: 16px;
      font-weight: 500;
      display: flex;
      align-items: center;
      gap: 10px;
    }
    .chat-header img {
      height: 22px;
      width: 22px;
    }
    .chat-box {
      flex: 1;
      padding: 16px;
      overflow-y: auto;
      display: flex;
      flex-direction: column;
      gap: 12px;
      background-color: #292C2D;
    }
    .message {
      max-width: 75%;
      padding: 12px 16px;
      border-radius: 18px;
      line-height: 1.5;
      font-size: 14px;
      word-wrap: break-word;
      background: #020505;
      color: #f1f1f1;
    }
    .user-msg {
      align-self: flex-end;
      background: #020505;
      color: #f1f1f1;
      border-bottom-right-radius: 4px;
    }
    .ai-msg {
      align-self: flex-start;
      border-bottom-left-radius: 4px;
    }
    .typing-indicator {
      align-self: flex-start;
      font-size: 13px;
      color: #ccc;
      font-style: italic;
      padding: 0 12px;
    }
    .chat-input {
      display: flex;
      border-top: 1px solid #444;
      background: #262626;
    }
    .chat-input input {
      flex: 1;
      padding: 14px;
      border: none;
      font-size: 14px;
      outline: none;
      background: #292C2D;
      color: white;
    }
    .chat-input button {
      background: #c39858;
      color: white;
      border: none;
      padding: 0 20px;
      cursor: pointer;
      font-weight: bold;
    }
    .chat-input button:hover {
      background: #2563eb;
    }
  </style>
</head>
<body>

  <div id="nameContainer" class="name-container">
	 <div class="chat-header">
      <img src="/static/Logo.svg" alt="logo" />
      Hairkatz AI Beta Test
    </div>
    <input type="text" id="nameInput" placeholder="Nama Anda..." />
    <button onclick="startChat()">Mulai Chat</button>
  </div>

  <div id="chatContainer" class="chat-container" style="display: none;">
	<div class="chat-header">
	  <img src="/static/Logo.svg" alt="logo" />
	  <div style="flex: 1;">Hairkatz AI Assistant</div>
	 <button onclick="reportChat()" style="background:none; border:none; color:#f87171; cursor:pointer; font-size:12px; padding: 4px;">
	  Laporkan Kekeliruan Percakapan
	</button>
	</div>
    <div id="chatBox" class="chat-box"></div>
    <div class="chat-input">
      <input id="messageInput" type="text" placeholder="Tulis pesan di sini..." onkeydown="handleEnter(event)" />
      <button onclick="sendMessage()">Kirim</button>
    </div>
  </div>

  <script>
    let participantName = '';
    const nameInput = document.getElementById('nameInput');
    const nameContainer = document.getElementById('nameContainer');
    const chatContainer = document.getElementById('chatContainer');
    const chatBox = document.getElementById('chatBox');
    const messageInput = document.getElementById('messageInput');
    let typingDiv = null;
    let typingInterval = null;

    function startChat() {
      const name = nameInput.value.trim();
      if (!name) return alert('Nama tidak boleh kosong');
      participantName = name;
      nameContainer.style.display = 'none';
      chatContainer.style.display = 'flex';
    }

    function appendMessage(content, role) {
      const div = document.createElement('div');
      div.className = 'message ' + (role === 'user' ? 'user-msg' : 'ai-msg');
      div.textContent = content;
      chatBox.appendChild(div);
      chatBox.scrollTop = chatBox.scrollHeight;
    }

    function showTyping() {
      typingDiv = document.createElement('div');
      typingDiv.className = 'typing-indicator';
      typingDiv.textContent = 'Hairo sedang mengetik';
      chatBox.appendChild(typingDiv);
      chatBox.scrollTop = chatBox.scrollHeight;

      let dots = '';
      typingInterval = setInterval(() => {
        dots = dots.length >= 3 ? '' : dots + '.';
        typingDiv.textContent = 'Hairo sedang mengetik' + dots;
        chatBox.scrollTop = chatBox.scrollHeight;
      }, 500);
    }

    function hideTyping() {
      if (typingInterval) clearInterval(typingInterval);
      if (typingDiv) {
        chatBox.removeChild(typingDiv);
        typingDiv = null;
        typingInterval = null;
      }
    }

    function sendMessage() {
      const msg = messageInput.value.trim();
      if (!msg) return;

      appendMessage(msg, 'user');
      messageInput.value = '';
      showTyping();

      fetch("/chat", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          message: msg,
          participant_name: participantName
        })
      })
      .then(res => res.json())
      .then(data => {
        hideTyping();
        appendMessage(data.message, 'ai');
      })
      .catch(err => {
        hideTyping();
        appendMessage("⚠️ Terjadi kesalahan: " + err.message, 'ai');
      });
    }

    function handleEnter(event) {
      if (event.key === "Enter") {
        sendMessage();
      }
    }

	function reportChat() {
	  if (!participantName) {
		alert("Nama pengguna belum tersedia.");
		return;
	  }
	
	  fetch("/chat-ui/report/" + encodeURIComponent(participantName), {
		method: "POST"
	  })
	  .then(res => res.json())
	  .then(data => {
		alert("✅ " + data.message);
	  })
	  .catch(err => {
		alert("❌ Gagal mengirim laporan: " + err.message);
	  });
	}
  </script>
</body>
</html>
`)
}
