
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

function postForm(url, formData, success, error) {
	return post(url, formData, 'application/x-www-form-urlencoded', null, null, success, error);
}

function reload() {
	window.location.reload();
}

function postFormAndReload(url, formData) {
	return post(url, formData, 'application/x-www-form-urlencoded', null, null, reload, reload);
}



function disableOwned(id) {
	postFormAndReload('/ui/owned/disable', "id=" + id);
}

function enableOwned(id) {
	postFormAndReload('/ui/owned/enable', "id=" + id);
}

function removeOwned(id) {
	postFormAndReload('/ui/owned/remove', "id=" + id);
}


function disableWatched(id) {
	postFormAndReload('/ui/watched/disable', "id=" + id);
}

function enableWatched(id) {
	postFormAndReload('/ui/watched/enable', "id=" + id);
}

function removeWatched(id) {
	postFormAndReload('/ui/watched/remove', "id=" + id);
}
