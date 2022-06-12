// websocket state
const socketConnecting = 0;
const socketOpen = 1;
const socketClosing = 2;
const socketClosed = 3;
var ws;

function launchWebsocketClient() {
    var username = document.getElementById("name").value;
    if (username.length <= 0) {
        alert("please, username")
        return
    }

    document.getElementById("name").value = "";
    connectToWebSocketServer(username);
}

function connectToWebSocketServer(username) {

    if ("WebSocket" in window) {
        ws = new WebSocket("ws://localhost:8080/chat")
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
            console.log(event.wasClean);
            logText("Connection has been closed");
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
    userContainer.classList.add("drag-and-drop", "user");
    userContainer.setAttribute('id', id);
    userContainer.style.backgroundColor = iconColor;

    var username = document.createElement('div');
    username.classList.add("user-name");
    username.innerHTML = name;

    userContainer.append(username);
    document.getElementById("users-space").append(userContainer);
}

function startmove(id) {
    var user = document.getElementById(id);

    user.onmousedown = function(event) {

        let shiftX = event.clientX - user.getBoundingClientRect().left;
        let shiftY = event.clientY - user.getBoundingClientRect().top;

        document.getElementById('users-space').append(user);

        moveAt(event.pageX, event.pageY);

        function moveAt(pageX, pageY) {
            var userid = user.id;
            const moved = {
                type: "move",
                addr: userid,
                position: {
                    pagex: pageX - shiftX,
                    pagey: pageY - shiftY,
                },
            };

            if (typeof(ws) != undefined && socketOpen === ws.readyState) {
                ws.send(JSON.stringify(moved));
            }
        }

        function onMouseMove(event) {
            moveAt(event.pageX, event.pageY);
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

function logText(text) {
    var currentdate = new Date();
    var datetime = "[" +
        currentdate.getHours() + ":" +
        currentdate.getMinutes() + ":" +
        currentdate.getSeconds() + "]";

    document.getElementById("log").innerHTML += datetime + " " + text + "<br/>";
}

function clearLog() {
    document.getElementById("log").innerHTML = "";
}
