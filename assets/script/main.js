// websocket state
const socketConnecting = 0;
const socketOpen = 1;
const socketClosing = 2;
const socketClosed = 3;
var ws;

function launchWebsocketClient() {
    document.getElementById("launch").style.display = "none";
    document.getElementById("close").style.display = "block";
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
        document.getElementById("own").value = json.to;
        appendNewUserIcon(json.to, "red", json.position);
        startmove(json.to);
    }
    if (json.type === "move") {
        movedClient(json.to, json.position);
    }
    if (json.type === "leaved") {
        removeUserIcon(json.to)
    }
    if (json.type === "private") {
        console.log(json)
    }

}

function removeUserIcon(id) {
    var ele = document.getElementById(id);
    ele.remove();
}

function positioning(ele, position) {
    ele.style.left = position.pagex + "px";
    ele.style.top = position.pagey + "px";
}

function movedClient(id, position) {
    var isExist = document.getElementById(id);
    if (isExist === null) {
        appendNewUserIcon(id, "black", position);
    }

    var client = document.getElementById(id);
    positioning(client, position)
}

function createElement(tagName, classArr, id) {
    var tag = document.createElement(tagName);
    classArr.forEach(className => {
        tag.classList.add(className);
    });

    if (id != undefined) {
        tag.setAttribute("id", id);
    }

    return tag;
}

function appendNewUserIcon(id, iconColor, position) {
    var userContainer = createElement('div', ["d-flex", "drag-and-drop"], id);
    positioning(userContainer, position)

    var userIcon = createElement("div", ["user"]);
    userIcon.style.backgroundColor = iconColor;

    var username = createElement('div', ["user-name"]);
    userIcon.append(username);
    userContainer.append(userIcon);

    if ("black" === iconColor) {
        var mailIcon = createElement('i', ["bi", "bi-envelope"]);
        userContainer.append(mailIcon);
        mailIcon.addEventListener("click", function(event) {
            var ele = event.target.parentNode.closest(".drag-and-drop");
            var address = ele.getAttribute("id");
            const myModalEl = document.getElementById('chatModal')
            const modal = new mdb.Modal(myModalEl)
            modal.show()
            setHeaderInfoAndClickAction(address);
        });
    }

    document.getElementById("users-space").append(userContainer);
}

function setHeaderInfoAndClickAction(address) {
    document.getElementsByClassName("modal-title")[0].innerHTML = address;

    document.getElementById("send").addEventListener("click", function() {
        var ms = document.getElementById("privatemsg").value;
        document.getElementById("privatemsg").value = "";
        var ownId = document.getElementById("own").value;
        var msg = {
            type: "private",
            to: address,
            from: ownId,
            msg: ms,
        }
        sendSocketServer(msg);
    })
}

function startmove(id) {
    var user = document.getElementById(id);

    user.onmousedown = function(event) {
        var shiftX = event.clientX - user.getBoundingClientRect().left;
        var shiftY = event.clientY - user.getBoundingClientRect().top;

        document.getElementById("users-space").append(user);
        moveAt(event.pageX, event.pageY);

        function moveAt(pageX, pageY) {
            var userid = user.id;
            const moved = {
                type: "move",
                to: userid,
                position: {
                    pagex: pageX - shiftX,
                    pagey: pageY - shiftY,
                },
            };
            sendSocketServer(moved);
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


function sendSocketServer(json) {
    if (typeof(ws) != undefined && socketOpen === ws.readyState) {
        ws.send(JSON.stringify(json));
    }
}

function closeWebsocketClient() {
    document.getElementById("users-space").innerHTML = "";
    document.getElementById("close").style.display = "none";
    document.getElementById("launch").style.display = "block";
    if (typeof(ws) != "undefined") ws.close(1000, "Work complete");;
}
