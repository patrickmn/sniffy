String.prototype.trunc = function(n) {
    return this.substr(0,n-1)+(this.length>n?"...":"");
};

////
// Page con-/destructors
////
var PAGES = {
    "": "", // Universal constructors
    "/auditor/dashboard": "auditor_dashboard",
    "/auditor/interceptor": "auditor_interceptor",
};

function getPage(url) {
    return PAGES[url];
};

var CONSTRUCTORS = {};
function addConstructor(url, f) {
    if (CONSTRUCTORS[url] == null) {
	CONSTRUCTORS[url] = [];
    };
    CONSTRUCTORS[url].push(f);
};

function runConstructors(url) {
    if (url != "") {
	runConstructors(""); // Run universal constructors
    };
    var page = getPage(url);
    var funcs = CONSTRUCTORS[page];
    if (funcs != null) {
	$.each(funcs, function(i, v) {
	    v();
	});
	funcs = [];
    };
};

// Destructors are ephemeral; added when the page is loaded,
// and executed and removed when the page is changed
var DESTRUCTORS = [];
function addDestructor(f) {
    DESTRUCTORS.push(f);
};

function runDestructors() {
    $.each(DESTRUCTORS, function(i, v) {
	v();
    });
    DESTRUCTORS = [];
};

////
// Navigation
////

var loc = window.location
if (loc.pathname != "/") {
    window.location = "/#!" + loc.pathname;
};

// TODO: Maybe make HTTP handlers that get e.g. "about" template, and "about_full"
//       which just does {{header}}"about"{{footer}}. Wouldn't even have to be JSON
function loadIntoNom($parent, urlFilter, cb) {
    var $nom = $parent.find("div#nom")
    $nom.fadeOut(200, function() {
	$nom.remove();
	$parent.hide().load(urlFilter, function(responseText, textStatus) {
	    $parent.fadeIn(200);
	    cb(textStatus == "error");
	});
    });
};

function getHash() {
    return window.location.hash.substring(2);
};

function setHash(x) {
    window.location.hash = "!" + x;
};

function getPath() {
    return getHash().split("?", 1)[0]
};

$(function() {
    var $content = $("section#site")

    // var $nav = $("nav");
    var $nav = $("body");
    $nav.delegate("a", "click", function() {
	var href = $(this).attr("href");
	if (href != "#" && href.substring(0, 7) != "http://" && href.substring(0, 8) != "https://") {
            setHash(href);
            return false;
	};
    });

    $(window).bind("hashchange", function(){
        var hash = getHash();
        if (hash) {
	    runDestructors();
	    loadIntoNom($content, hash + " section#site > #nom", function(error) {
		if (error) {
		    $content.append('<div id="nom"><p>The page could not be loaded right now. Please try to <a href="#" onClick="window.location.reload();">reload the page</a>.</p></div>');
		} else {
	            runConstructors(getPath());
		};
	    });
	    $nav.find("a").removeClass("active");
            $nav.find("nav a[href="+escapeMeta(hash)+"]").addClass("active");
        };
    });
    $(window).trigger("hashchange");
});

addConstructor("", function() {
    getProxyServerSelector().change(function() {
        setHash(getPath() + "?ps=" + getProxyServerId());
    });
});

////
// Effects
////

function scrollTo(elem) {
    return elem.animate({scrollTop: 0}, "slow");
};

function scrollTop() {
    return scrollTo($("html, body"));
};

function fadeIn($x, fadeDur, animDur, cb) {
    if (fadeDur == null) {
	fadeDur = 500;
    };
    if (animDur == null) {
	animDur = 3000;
    };
    var origbg = $x.css("background-color");
    $x.css({
	"visibility": "hidden",
	"opacity": 100,
	"background-color": "#FEE9CC",
    });
    $x.css({
	"visibility": "visible",
	"opacity": 0,
    }).fadeTo(fadeDur, 1, function() {
	$x.animate({backgroundColor: origbg}, animDur, "swing", function() {
	    if (cb != null) {
		cb();
	    };
	});
    });
};

function fadeOut($x, dur, cb) {
    if (dur == null) {
	dur = 500;
    };
    $x.fadeOut(dur, "swing", function() {
	if (cb != null) {
	    cb();
	};
    });
};


////
// Utility
////

var escapeChars = {
    "&": "amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#x27;",
    "/": "&#x2F;",
};

function escape(s) {
    if (s != null) {
	for (var k in escapeChars) {
	    s = s.replace(k, escapeChars[k]);
	};
    };
    return s;
};

function escapeMeta(s) {
    s = s.replace(/([ !"#$%&'()*+,.\/:;<=>?@[\\\]^`{|}~])/g,'\\$1')
    return s;
};

function sanitizeRequest(r) {
    r.Method = escape(r.Method);
    r.URL.String = escape(r.URL.Raw);
    r.Proto = escape(r.Proto);
    if (r.Header != null) {
	var newHeader = {};
	$.each(r.Header, function(k, v) {
	    newHeader[escape(k)] = escape(v.toString());
	});
	r.Header = newHeader;
    };
    if (r.TransferEncoding != null) {
	var newTransferEncoding = [];
	$.each(r.TransferEncoding, function(i, v) {
	    newTransferEncoding[i] = escape(v);
	});
	r.TransferEncoding = newTransferEncoding;
    };
    r.Host = escape(r.Host);
    r.RemoteAddr = escape(r.RemoteAddr);
    if (r.Response != null) {
	r.Response = sanitizeResponse(r.Response);
    };
    return r
};

function sanitizeResponse(r) {
    r.Status = escape(r.Status);
    r.Proto = escape(r.Proto);
    if (r.Header != null) {
	var newHeader = {};
	$.each(r.Header, function(k, v) {
	    newHeader[escape(k)] = escape(v.toString());
	});
	r.Header = newHeader;
    };
    if (r.TransferEncoding != null) {
	var newTransferEncoding = [];
	$.each(r.TransferEncoding, function(i, v) {
	    newTransferEncoding[i] = escape(v);
	});
	r.TransferEncoding = newTransferEncoding;
    };
    return r
};

function headerToList(h) {
    var html = "<ul>"
    $.each(h, function(i, v) {
	html += "<li>"+i+": "+v+"</li>";
    });
    html += "</ul>";
    return html;
};

function headerToTable(h) {
    var html = '<table class="condensed-table bordered-table" style="table-layout: fixed; word-wrap: break-word;"><thead><tr><th width="20%">Name</th><th width="80%">Value</th></tr></thead><tbody>'
    $.each(h, function(i, v) {
	html += "<tr><td>"+i+"</td><td>"+v+"</td></tr>";
    });
    html += "</tbody></table>";
    return html;
};

function urlToTable(u) {
    var html = '<table class="condensed-table bordered-table" style="table-layout: fixed; word-wrap: break-word;"><thead><tr><th width="20%">Name</th><th width="80%">Value</th></tr></thead><tbody>';
    // TODO: Use medialize.github.com/URI.js instead?
    var params = $.url(u).param();
    var found = false;
    $.each(params, function(i, v) {
	found = true;
	html += "<tr><td>"+i+"</td><td>"+v+"</td></tr>";
    });
    html += "</tbody></table>";
    if (found) {
	return html;
    };
};

function encodingToList(e) {
    var html = "<ul>"
    $.each(e, function(i, v) {
	html += "<li>"+v+"</li>";
    });
    html += "</ul>";
    return html;
};

function emptyRequest(s) {
    return {
	Id: 0,
	Time: 0,
	Method: "GET",
	URL: {
	    Raw: "",
	    String: "",
	},
	Proto: "HTTP/1.1",
	Header: {},
	// TransferEncoding:
	Host: "",
    };
};

function emptyResponse(s) {
    msg = "No response recorded";
    if (s != null) {
	msg = s;
    };
    return {
	Id: "N/A",
	Time: 0,
	Status: msg,
	StatusCode: 0,
	Proto: "N/A",
	Header: "N/A",
	ContentLength: "N/A",
	TransferEncoding: "N/A",
	Close: "N/A",
    };
};

function getProxyServerSelector() {
    return $("select#proxyserver")

};

function getProxyServerId() {
    return getProxyServerSelector().find("option:selected").val();
};

////
// Auditor/Interceptor
////

function getDetailRow(id, cb) {
    $.ajax({
	url: "/auditor/json/getrequest",
	data: {
	    "id": id,
	    "type": "full",
	},
	dataType: "json",
	contentType: "application/json",
	success: function(data) {
	    if (data.r != null) {
		var v = sanitizeRequest(data.r);
		var detailreqheaderhtml = headerToTable(v.Header);
		var detailrequrlhtml = ""
		var urldetails = urlToTable(v.URL.Raw);
		if (urldetails != null) {
		    detailrequrlhtml = "<br /><br /><p>Parameters:</p>"+urldetails;
		}
		var detailresheaderhtml = ""
		if (v.Response == null) {
		    v.Response = emptyResponse();
		} else {
		    detailresheaderhtml = headerToTable(v.Response.Header);
		};
		var detailreqencodinghtml = ""
		if (v.TransferEncoding != null) {
		    detailreqencodinghtml = encodingToList(v.TransferEncoding);
		};
		var detailresencodinghtml = ""
		if (v.Response != null && v.TransferEncoding != null) {
		    detailresencodinghtml = encodingToList(v.Response.TransferEncoding);
		};
		var detailhtml = '\
<tr id="details-'+v.Id+'">\
    <td colspan="4">\
	<table class="condensed-table" style="table-layout: fixed; word-wrap: break-word;">\
	<thead>\
	    <tr>\
		<th>Request</th>\
		<th>Response</th>\
	    </tr>\
	</thead>\
	<tbody>\
	    <tr>\
		<td>\
		    <table class="zebra-striped bordered-table condensed-table" style="table-layout: fixed; word-wrap: break-word;">\
		    <thead></thead>\
		    <tbody>\
			<tr>\
			    <td width="20%">ID</td>\
			    <td width="80%">'+v.Id+'</td>\
			</tr>\
			<tr>\
			    <td>Time</td>\
			    <td>'+new Date(v.Time * 1000)+'</td>\
			</tr>\
			<tr>\
			    <td>Method</td>\
			    <td>'+v.Method+'</td>\
			</tr>\
			<tr>\
			    <td>URL</td>\
			    <td><a href="'+v.URL.String+'" target="_blank">'+v.URL.String+'</a>'+detailrequrlhtml+'</td>\
			</tr>\
			<tr>\
			    <td>Protocol</td>\
			    <td>'+v.Proto+'</td>\
			</tr>\
			<tr>\
			    <td>Header</td>\
			    <td>'+detailreqheaderhtml+'</td>\
			</tr>\
			<tr>\
			    <td>Content Length</td>\
			    <td>'+v.ContentLength+'</td>\
			</tr>\
			<tr>\
			    <td>Transfer Encoding</td>\
			    <td>'+detailreqencodinghtml+'</td>\
			</tr>\
			<tr>\
			    <td>Server</td>\
			    <td>'+v.Host+'</td>\
			</tr>\
			<tr>\
			    <td>Client</td>\
			    <td>'+v.RemoteAddr+'</td>\
			</tr>\
			<tr>\
			    <td>SSL</td>\
			    <td>'+v.TLSHandshakeDone+'</td>\
			</tr>\
			<tr>\
			    <td>Actions</td>\
			    <td>\
				<button id="emulate-'+v.Id+'" class="btn">Emulate</button>\
				<button id="analyze-'+v.Id+'" class="btn">Analyze</button>\
				<button id="allow-'+v.Id+'" class="btn">Allow</button>\
				<button id="deny-'+v.Id+'" class="btn">Deny</button>\
			    </td>\
			</tr>\
		    </tbody>\
		    </table>\
		</td>\
		<td>\
		    <table class="zebra-striped bordered-table condensed-table" style="table-layout: fixed; word-wrap: break-word;">\
		    <thead></thead>\
		    <tbody>\
			<tr>\
			    <td width="20%">ID</td>\
			    <td width="80%">'+v.Response.Id+'</td>\
			</tr>\
			<tr>\
			    <td>Time</td>\
			    <td>'+new Date(v.Response.Time * 1000)+'</td>\
			</tr>\
			<tr>\
			    <td>Status</td>\
			    <td>'+v.Response.Status+'</td>\
			</tr>\
			    <td>Status Code</td>\
			    <td>'+v.Response.StatusCode+'</td>\
			</tr>\
			<tr>\
			    <td>Protocol</td>\
			    <td>'+v.Response.Proto+'</td>\
			</tr>\
			<tr>\
			    <td>Header</td>\
			    <td>'+detailresheaderhtml+'</td>\
			</tr>\
			<tr>\
			    <td>Content Length</td>\
			    <td>'+v.Response.ContentLength+'</td>\
			</tr>\
			<tr>\
			    <td>Transfer Encoding</td>\
			    <td>'+detailresencodinghtml+'</td>\
			</tr>\
			<tr>\
			    <td>Closed connection</td>\
			    <td>'+v.Response.Close+'</td>\
			</tr>\
		    </tbody>\
		    </table>\
		</td>\
	    </tr>\
	</tbody>\
	</table>\
    </td>\
</tr>';
		cb(detailhtml);
	    };
	},
    });
};

function showEmulateModal(r) {
    var $modal = $('\
<div id="emulate-modal-'+r.Id+'" class="modal hide fade xlarge">\
    <div class="modal-header">\
        <a href="#" class="close">&times;</a>\
        <h3>Emulate Request</h3>\
    </div>\
    <form id="emulate-modal-'+r.Id+'-form">\
    <div class="modal-body">\
	<fieldset>\
	    <div class="clearfix">\
		<label for="method">Method</label>\
		<div class="input">\
		    <select id="method" name="method">\
			<option>HEAD</option>\
			<option>GET</option>\
			<option>POST</option>\
			<option>PUT</option>\
			<option>DELETE</option>\
			<option>TRACE</option>\
			<option>OPTIONS</option>\
			<option>CONNECT</option>\
			<option>PATCH</option>\
			<option value="custom">Custom...</option>\
		    </select>\
		    <input id="methodcustom" name="methodcustom" type="text" />\
		</div>\
	    </div>\
	    <div class="clearfix">\
		<label for="url">URL</label>\
		<div class="input">\
		    <textarea id="url" name="url" class="xxlarge">'+r.URL.String+'</textarea>\
		</div>\
	    </div>\
	    <div class="clearfix">\
		<label for="proto">Protocol</label>\
		<div class="input">\
		    <input id="proto" name="proto" type="text" value="'+r.Proto+'" />\
		</div>\
	    </div>\
	    <div id="header" class="clearfix"></div>\
	    <div id="transferencoding" class="clearfix"></div>\
	    <div class="clearfix">\
		<label for="host">Hostname</label>\
		<div class="input">\
		    <input id="host" name="host" type="text" value="'+r.Host+'" />\
		</div>\
	    </div>\
	    <div class="clearfix">\
		<label for="body">Body/Data</label>\
		<div class="input">\
		    <textarea id="body" name="body" class="xxlarge"></textarea>\
		</div>\
	    </div>\
	</fieldset>\
    </div>\
    <div class="modal-footer">\
        <input id="submitemulate" name="submitemulate" type="submit" class="btn primary" value="Emulate" />\
        <input id="submitview" name="submitview" type="submit" class="btn secondary" value="View" />\
	<p>"Emulate" to send the request; "View" to send the request and retrieve the response in the browser. (Note that the page will be able to detect Sniffy via the DOM.)</p>\	<input id="emulate" name="emulate" type="hidden" value="'+r.Id+'" />\
    </div>\
    </form>\
</div>\
');
    $modal.modal({
	backdrop: true,
	keyboard: true,
	show: true,
    });
    var methodcustom = $modal.find("input#methodcustom");
    methodcustom.hide();
    // TODO: Set initial value if req.Method was custom
    var methodselect = $modal.find("select#method");
    methodselect.val(r.Method);
    methodselect.change(function() {
	if ($(this).val() == "custom") {
	    methodcustom.show();
	} else {
	    methodcustom.hide();
	};
    });

    var header = $modal.find("div#header");
    header.html('\
<label for="headertable">Header</label>\
<div class="input">\
    <table id="headertable" class="zebra-striped bordered-table condensed-table" style="table-layout: fixed; word-wrap: break-word;">\
    <thead>\
	<tr>\
	    <th width="20%">Name</th>\
	    <th width="74%">Value</th>\
	    <th width="6%"></th>\
	</tr>\
    </thead>\
    <tbody>\
	<tr><td colspan="3"><button id="add" class="btn small">+</button></td></tr>\
    </tbody>\
    </table>\
</div>\
');
    var headerrows = "";
    function headerRow(name, val) {
	var $row = $('<tr><td><input type="text" value="'+name+'" style="width: 90%;" /></td><td><input type="text" value="'+val+'" style="width: 97%;" /></td><td><button id="remove" class="btn small">-</button></td></tr>');
	$row.find("button#remove").click(function() {
	    $row.remove();
	    return false;
	});
	return $row
    };
    function insertRow($row) {
	header.find("table > tbody > tr:last").before($row);
    };
    $.each(r.Header, function(ok, ov) {
	insertRow(headerRow(ok, ov));
    });
    header.find("button#add").click(function() {
	var $row = headerRow("", "");
	insertRow($row);
	return false;
    });

    var $modalform = $modal.find("form");
    $modalform.submit(function() {
	var data = $(this).serializeArray();
	var first = true;
	var cur;
	$.each(header.find("table > tbody > tr > td > input"), function() {
	    var val = $(this).val()
	    if (first) {
		cur = val;
	    } else {
		if (cur != "") {
		    data.push({
			name: "HEADER:"+cur,
			value: val,
		    });
		};
	    };
	    first = !first;
	});
	$.ajax({
	    url: "/auditor/json/makerequest?ps=" + getProxyServerId(),
	    type: "POST",
	    data: data,
	    dataType: "json",
	    success: function(data) {
		$modal.modal("hide");
		scrollTop();
	    },
	});
	return false;
    });
};

function showAnalyzeModal(r) {
    var $modal = $('\
<div id="analyze-modal-'+r.Id+'" class="modal hide fade xlarge">\
    <div class="modal-header">\
        <a href="#" class="close">&times;</a>\
        <h3>Analyze Request</h3>\
    </div>\
    <div class="modal-body">\
	<p>Hi!</p>\
    </div>\
    <div class="modal-footer">\
	<input id="analyze" name="analyze" type="hidden" value="'+r.Id+'" />\
        <input id="submitanalyze" name="submitanalyze" type="submit" class="btn primary" value="Analyze" />\
        <input id="submitview" name="submitview" type="submit" class="btn secondary" value="View" />\
    </div>\
    </form>\
</div>\
');
    $modal.modal({
	backdrop: true,
	keyboard: true,
	show: true,
    });
};

function insertRequestRow(r, after, cb) {
    var time = new Date(r.Time * 1000);
    var reqhtml = '\
<tr class="request" id="'+r.Id+'">\
<td>'+time.toLocaleTimeString()+'</td>\
<td>'+r.RemoteAddr+'</td>\
<td>'+r.Method+'</td>\
<td>'+r.URL.String.trunc(100)+'</td>\
</tr>';
    after.after(reqhtml);
    var $cur = $("tr#"+r.Id);
    var $details;
    // TODO: Use $(filter).delegate("tr", click, function() {}) instead -- more efficient
    $cur.click(function() {
	// TODO: Check if an existing detail row has Response info (if user clicks really fast initially)
	if ($details != null) {
	    $details.toggle();
	} else {
	    getDetailRow(r.Id, function(detailhtml) {
		$cur.after(detailhtml);
		$details = $("tr#details-"+r.Id);
		$("button#emulate-"+r.Id).click(function() {
		    showEmulateModal(r);
		});
		$("button#analyze-"+r.Id).click(function() {
		    showAnalyzeModal(r);
		});
	    });
	};
    });
    cb($cur);
};

function getRequests(psId, since, cb) {
    $.ajax({
	url: "/auditor/json/getrequests",
	data: {
	    ps: psId,
	    since: since,
	    type: "summary",
	},
	dataType: "json",
	contentType: "application/json",
	// cache: false,
	success: function(data) {
	    if (data.rs != null) {
		$.each(data.rs, function(i, v) {
		    r = sanitizeRequest(v);
		});
	    }
	    since = data.since;
	    cb(since, data.rs, data.queue); // Will the sanitized ones be returned?
	},
	error: function() {
	    cb(since, null, null);
	},
    });
};

var POLLING_INTERVAL = 100;
var POLLING_PAUSED = false;
var POLLING_STOP = false;
function pollAndInsertRequests(psId, since, interval, insertAfter) {
    function retry() {
	setTimeout(function() {pollAndInsertRequests(psId, since, interval, insertAfter);}, interval)
    };
    if (POLLING_PAUSED) {
	// TODO: Make it so we don't keep polling, but keep "Pause" button state "atomic"
	retry();
	return;
    } else if (POLLING_STOP) {
	POLLING_STOP = false;
	return;
    };
    getRequests(psId, since, function(newSince, rs, queue) {
	if (rs != null) {
	    $.each(rs, function(i, r) {
		insertRequestRow(r, insertAfter, function(row) {
		    if (queue != null && jQuery.inArray(r.Id, queue)) {
			// row.addClass("important");
			row.css("background-color", "red");
		    };
		    if (since > 0) {
			fadeIn(row);
		    };
		});
	    });
	    since = newSince;
	};
	retry();
    });
};

addConstructor("auditor_interceptor", function() {
    addDestructor(function() {
	POLLING_STOP = true;
    });
    var psId = getProxyServerId();
    pollAndInsertRequests(psId, 0, POLLING_INTERVAL, $("table#requests tr:first"));

    // New request button
    var newrequestbutton = $("button#newrequest");
    newrequestbutton.click(function() {
	showEmulateModal(emptyRequest());
    });

    // Pause polling button
    var pausepollingbutton = $("button#pausepolling");
    pausepollingbutton.click(function() {
	POLLING_PAUSED = !POLLING_PAUSED;
    });

    // Show/hide all button
    var showhideallbutton = $("button#showhideall");
    function showHideAll() {
	$("tr").trigger("click");
    };
    showhideallbutton.click(showHideAll);

    // Clear requests button
    var clearbutton = $("button#clearrequests");
    function clearRequests() {
	clearbutton.button("loading")
	$.ajax({
	    url: "/auditor/json/deleterequests",
	    data: {
		"ps": getProxyServerId(),
	    },
	    success: function(data) {
		$("table#requests tr:gt(0)").remove();
		clearbutton.button("reset");
	    },
	});
    };
    clearbutton.click(clearRequests);

    // $(function() {
    // 	$("table#detailinner").tablesorter();
    // });

    // Log Requests button
    var logrequestsbutton = $("button#togglelogrequests");
    function toggleLogRequests() {
	$.ajax({
	    url: "/auditor/json/toggle",
	    data: {
		"option": "logrequests",
	    },
	    success: function(data) { logrequestsbutton.button("toggle"); },
	});
    };
    logrequestsbutton.click(toggleLogRequests);
    if (logrequestsbutton.hasClass("on")) {
	logrequestsbutton.button("toggle");
    };

    // Intercept SSL button
    var interceptbutton = $("button#toggleinterceptssl");
    function toggleInterceptSSL() {
	$.ajax({
	    url: "/auditor/json/toggle",
	    data: {
		"option": "interceptssl",
	    },
	    success: function(data) { interceptbutton.button("toggle"); },
	});
    };
    interceptbutton.click(toggleInterceptSSL);
    if (interceptbutton.hasClass("on")) {
	interceptbutton.button("toggle");
    };

    // Moderate requests button
    var modbutton = $("button#togglemoderation");
    function toggleModeration() {
	$.ajax({
	    url: "/auditor/json/toggle",
	    data: {
		"option": "moderaterequests",
	    },
	    success: function(data) { modbutton.button("toggle"); },
	});
    };
    modbutton.click(toggleModeration);
    if (modbutton.hasClass("on")) {
	modbutton.button("toggle");
    };
});
