'use strict';

var inits = [];
var bindings = [];

// Called before document ready to bind events:
// sel = selector
function bind(sel, ev, cb) {
	bindings.push({sel: sel, ev: ev, cb: cb});
}

// Call the callback on document ready:
function oninit(cb) {
	inits.push(cb);
}

document.onreadystatechange = function () {
    if (document.readyState == "complete")
        init();
};


// utility functions:

String.prototype.endsWith = function (e) {
    return this.substring(this.length - e.length, this.length) == e;
}

Array.prototype.contains = function (item) {
    for (var i = 0; i < this.length; ++i)
        if (this[i] === item) return true;
    return false;
}


function bindEvent(target, type, listener) {
    if (target.addEventListener !== undefined)
        target.addEventListener(type, listener, false);
    else
        target.attachEvent('on' + type, listener);
}

// XMLHttpRequest helpers

function get(url, before, after, success, error) {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', url, true);

    xhr.onreadystatechange = function () {
        if (this.readyState != 4) return;
        try {
            if (this.status != 200) {
                error(this.responseText);
            } else {
                success(this.responseText);
            }
        } catch (ex) { }

		if (after) after();
    };

	if (before) before();
    xhr.send();

    return xhr;
}

function post(url, data, contentType, before, after, success, error) {
    var xhr = new XMLHttpRequest();
    xhr.open('POST', url, true);
	xhr.setRequestHeader("Content-Type", contentType);

    xhr.onreadystatechange = function () {
        if (this.readyState != 4) return;
        try {
            if (this.status != 200) {
                error(this.responseText);
            } else {
                success(this.responseText);
            }
        } catch (ex) { }

		if (after) after();
    };

	if (before) before();
    xhr.send(data);

    return xhr;
}

function getJson(url, success, error) {
	return get(url, null, null, function(txt) { success(JSON.parse(txt)); }, function(txt) { error(JSON.parse(txt)); });
}

function postJson(url, json, success, error) {
	return post(url, JSON.stringify(json), "application/json", null, null, function(txt) { success(JSON.parse(txt)); }, function(txt) { error(JSON.parse(txt)); });
}

function reload(newurl) {
	if (newurl === undefined) {
		window.location.reload();
		return;
	}

	window.location.href = newurl;
}
function byid(id) { return document.getElementById(id); }
function make(tag) { return document.createElement(tag); }
function text(text) { return document.createTextNode(text); }
function setAttribute(el, prop, value) {
    el.setAttribute(prop, value);
    return el;
}
function removeAttribute(el, prop) {
    el.removeAttribute(prop);
    return el;
}
function setStyle(el, prop, value) {
    if (el.style.setProperty !== undefined)
        el.style.setProperty(prop, value);
    else
        el.style.setAttribute(prop, value);
    return el;
}

// Alternatively, try `visibility` style.
function hide(id) { return setStyle(byid(id), 'display', 'none'); }
function show(id) {
    var obj = byid(id);

    var disp = 'block';
    if (obj.tagName) {
        if (obj.tagName === 'A')
            disp = 'inline';
        else if (obj.tagName === 'TD')
            disp = 'table-cell';
    }

    return setStyle(obj, 'display', disp);
}
function toggle(id) {
    var obj = byid(id);
    if (obj.style.display === 'none') {
        show(id);
        return true;
    } else {
        hide(id);
        return false;
    }
}

function enable(id, enabled) {
	var el = byid(id);
	if (el === null) return;

	if (enabled) {
		removeAttribute(el, "disabled");
	} else {
		setAttribute(el, "disabled", "disabled");
	}
}

// get or set value of `id` input element:
function v(id, setValue) {
    var el = byid(id);
    if (el === null) return null;

    if (el.getAttribute('type') === 'number') {
        // NOTE(jsd): The try/catch is for FF failure.
        try {
            if (setValue !== undefined)
                el.valueAsNumber = setValue;
            if (isNaN(el.valueAsNumber))
                return el.value;
            return el.valueAsNumber;
        } catch (e) {
            // default to using `value`:
            if (setValue !== undefined)
                el.value = setValue;
            return el.value;
        }
    }

    // Support for checkboxes with true/false values for checked/unchecked:
    if (el.getAttribute('type') === 'checkbox') {
        if (setValue !== undefined) {
            if (!!setValue)
                el.setAttribute('checked', 'checked');
            else
                el.removeAttribute('checked');
        }
        return el.checked;
    }

    if (setValue !== undefined)
        el.value = setValue;
    return el.value;
}

function tryParseInt(i) {
    try {
        return parseInt(i, 10);
    } catch (e) {
        return null;
    }
}


// Document is ready:
function init() {
	for (var i = 0; i < inits.length; i++) {
		var cb = inits[i];

		cb();
	}

	// Apply event bindings to elements found by selector:
	for (var i = 0; i < bindings.length; i++) {
		var b = bindings[i];

		var list = document.querySelectorAll(b.sel);
		for (var j = 0; j < list.length; j++) {
			bindEvent(list[j], b.ev, b.cb);
		}
	}
}

function standardJsonErrorHandler(rsp) {
	alert(rsp.error);
}
