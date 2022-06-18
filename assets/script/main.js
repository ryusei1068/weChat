// websocket state
const socketConnecting = 0;
const socketOpen = 1;
const socketClosing = 2;
const socketClosed = 3;
var ws;
var username = "";

function launchWebsocketClient() {
    username = document.getElementById("name").value;
    if (username.length <= 0) {
        alert("please, username")
        return
    }

    document.getElementById("name").value = "";
    connectToWebSocketServer(username);
}

function connectToWebSocketServer(username) {

    if ("WebSocket" in window) {
        ws = new WebSocket(`ws://${document.location.host}/chat`)
        ws.onopen = function() {
            document.getElementById("connectionState").innerHTML = "Connected";
            document.getElementById("connectionState").style.color = "green";
        };

        ws.onmessage = function(evt) {
            // logText("Message received from websocket : " + evt.data);
            var jsondata = JSON.parse(evt.data);
            if (jsondata.type === "newclient") {
                appendNewUserIcon(jsondata.addr, "red", username);
                startmove(jsondata.addr);
            }
            if (jsondata.type === "move") {
                movedClient(jsondata.addr, jsondata.position.pagex, jsondata.position.pagey);
            }
            if (jsondata.type === "leaved") {
                removeUserIcon(jsondata.addr)
            }
        };

        ws.onclose = function(event) {
            document.getElementById("connectionState").innerHTML = "Disconnected";
            document.getElementById("connectionState").style.color = "red";
            alert("Connection has been closed");
            if (!event.wasClean) {
                location.reload();
            }
        };

    } else {
        alert("WebSocket is NOT supported by your Browser!");
    }
}

function removeUserIcon(id) {
    var ele = document.getElementById(id);
    ele.remove();
}

function movedClient(id, pagex, pagey) {
    var isExist = document.getElementById(id);

    if (isExist === null) {
        appendNewUserIcon(id, "black");
    }

    var client = document.getElementById(id);

    client.style.left = pagex + "px";
    client.style.top = pagey + "px";
}

function appendNewUserIcon(id, iconColor, name) {
    var userContainer = document.createElement('div');
    userContainer.classList.add("d-flex", "drag-and-drop");
    userContainer.setAttribute("id", id)

    var userIcon = document.createElement("div");
    userIcon.classList.add("user");
    userIcon.style.backgroundColor = iconColor;

    var username = document.createElement('div');
    username.classList.add("user-name");
    username.innerHTML = name;
    userIcon.append(username);
    userContainer.append(userIcon);

    if ("black" === iconColor) {
        mailIcon = document.createElement('i');
        mailIcon.classList.add('bi', 'bi-envelope');
        userContainer.append(mailIcon);
        mailIcon.addEventListener("click", openModalWindow);
    }

    document.getElementById("users-space").append(userContainer);
}


function openModalWindow(event) {
    var chatWindow = document.getElementById("modal");
    console.log(event.target.parentNode.id);
    var container = document.createElement("div");
    document.getElementById("users-space").style.display = "none";

    var inputtag = document.createElement("input");

    var sendButton = document.createElement('button');
    sendButton.classList.add("btn", "btn-outline-primary");
    sendButton.innerHTML = "send";

    var closebutton = document.createElement('button');
    closebutton.classList.add("btn", 'btn-primary');
    closebutton.innerHTML = "closeModal";

    chatWindow.append(inputtag, sendButton, closebutton);
    closebutton.addEventListener('click', closeModalWindow);
    sendButton.addEventListener("click", function() {
        var msg
    })
}

function closeModalWindow(event) {
    document.getElementById("users-space").style.display = "block";

    document.getElementById('modal').innerHTML = '';
}


function startmove(id) {
    var user = document.getElementById(id);

    user.onmousedown = function(event) {

        let shiftX = event.clientX - user.getBoundingClientRect().left;
        let shiftY = event.clientY - user.getBoundingClientRect().top;

        document.getElementById("users-space").append(user);

        moveAt(event.pageX, event.pageY);

        function moveAt(pageX, pageY) {
            var userid = user.id;

            const windowWidth = document.documentElement.clientWidth;
            const windowHeight = document.documentElement.clientHeight;
            const moved = {
                type: "move",
                addr: userid,
                position: {
                    pagex: pageX - shiftX,
                    pagey: pageY - shiftY,
                    height: windowHeight,
                    width: windowWidth,
                },

            };

            if (typeof(ws) != undefined && socketOpen === ws.readyState) {
                ws.send(JSON.stringify(moved));
            }
        }

        function onMouseMove(event) {
            moveAt(event.pageX, event.pageY);

            user.hidden = true;
            let elemBelow = document.elementFromPoint(event.clientX, event.clientY);
            user.hidden = false;

            if (!elemBelow) return;
        }
        document.addEventListener('mousemove', onMouseMove);

        user.onmouseup = function() {
            document.removeEventListener('mousemove', onMouseMove);
            user.onmouseup = null;
        };
    };

    user.ondragstart = function() {
        return false;
    };
}


function closeWebsocketClient() {
    document.getElementById("users-space").innerHTML = "";
    if (typeof(ws) != "undefined") ws.close(1000, "Work complete");;
}
