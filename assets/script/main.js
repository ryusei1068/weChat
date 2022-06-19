// websocket state
const socketConnecting = 0;
const socketOpen = 1;
const socketClosing = 2;
const socketClosed = 3;
var ws;
var username = "";

function launchWebsocketClient() {
    //    username = document.getElementById("name").value;
    //    if (username.length <= 0) {
    //        alert("please, username")
    //        return
    //    }
    //
    //    document.getElementById("name").value = "";
    connectToWebSocketServer();
}

function connectToWebSocketServer() {

    if ("WebSocket" in window) {
        ws = new WebSocket(`ws://${document.location.host}/chat`)
        ws.onopen = function() {
            document.getElementById("connectionState").innerHTML = "Connected";
            document.getElementById("connectionState").style.color = "green";
        };

        ws.onmessage = function(event) {
            var json = JSON.parse(event.data);
            handlingJson(json);
        };

        ws.onclose = function(event) {
            document.getElementById("connectionState").innerHTML = "Disconnected";
            document.getElementById("connectionState").style.color = "red";
            console.log(event.wasClean);
        };

    } else {
        alert("WebSocket is NOT supported by your Browser!");
    }
}

function handlingJson(json) {
    if (json.type === "newclient") {
        appendNewUserIcon(json.addr, "red", json.position);
        startmove(json.addr);
    }
    if (json.type === "move") {
        movedClient(json.addr, json.position);
    }
    if (json.type === "leaved") {
        removeUserIcon(json.addr)
    }
    if (json.type === "private") {
        console.log(json)
    }

}

function removeUserIcon(id) {
    var ele = document.getElementById(id);
    ele.remove();
}

function movedClient(id, position) {
    var isExist = document.getElementById(id);

    if (isExist === null) {
        appendNewUserIcon(id, "black", position);
    }

    var client = document.getElementById(id);

    client.style.left = position.pagex + "px";
    client.style.top = position.pagey + "px";
}

function appendNewUserIcon(id, iconColor, position) {
    var userContainer = document.createElement('div');
    userContainer.classList.add("d-flex", "drag-and-drop");
    userContainer.setAttribute("id", id)
    userContainer.style.left = position.pagex + "px";
    userContainer.style.top = position.pagey + "px";

    var userIcon = document.createElement("div");
    userIcon.classList.add("user");
    userIcon.style.backgroundColor = iconColor;

    var username = document.createElement('div');
    username.classList.add("user-name");
    username.innerHTML = "test"
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
    var address = event.target.parentNode.id;
    document.getElementById("users-space").style.display = "none";

    var inputtag = document.createElement("input");
    inputtag.setAttribute("id", "privatemsg");
    var sendButton = document.createElement('button');
    sendButton.classList.add("btn", "btn-outline-primary");
    sendButton.innerHTML = "send";

    var closebutton = document.createElement('button');
    closebutton.classList.add("btn", 'btn-primary');
    closebutton.innerHTML = "closeModal";

    chatWindow.append(inputtag, sendButton, closebutton);
    closebutton.addEventListener('click', closeModalWindow);
    sendButton.addEventListener("click", function() {
        var ms = document.getElementById("privatemsg").value;
        var msg = {
            type: "private",
            addr: address,
            msg: ms,
        }
        sendSocketServer(msg);
    })
}

function closeModalWindow() {
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

            const moved = {
                type: "move",
                addr: userid,
                position: {
                    pagex: pageX - shiftX,
                    pagey: pageY - shiftY,
                },
            };
            sendSocketServer(moved);
        }

        function onMouseMove(event) {
            moveAt(event.pageX, event.pageY);

            //            user.hidden = true;
            //            let elemBelow = document.elementFromPoint(event.clientX, event.clientY);
            //            user.hidden = false;
            //            if (!elemBelow) return;
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


function sendSocketServer(json) {
    if (typeof(ws) != undefined && socketOpen === ws.readyState) {
        ws.send(JSON.stringify(json));
    }
}

function closeWebsocketClient() {
    document.getElementById("users-space").innerHTML = "";
    if (typeof(ws) != "undefined") ws.close(1000, "Work complete");;
}
