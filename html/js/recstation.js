(function() {
    var BASE_URL = '/api/v1';

    var is_recording = false;
    var recordingStart = null;

    var counterElem = $('#transport-counter');
    var counterInterval = null;
    var counterRefresh = 1000 / 24;
    var counter;

    var recordElem = $('#transport-record');
    var recordRefresh = 628;
    var recordInterval = null;

    var current_hostname = null;

    var sinks = {};
    var sinkNames = [];

    function setHostname(hostname) {
        if (current_hostname == hostname) {
            return;
        }

        current_hostname = hostname;
        $('#hostname').text(hostname);
        $('.hostname-container').show();
    }

    function now() {
        return new Date().getTime() / 1000;
    }

    function updateCounter(clear) {
        var dur = 0;

        if (!clear) {
            dur = now() - recordingStart;
        }

        var minutes = Math.floor(dur / 60.0)
        var seconds = Math.floor(dur - 60*minutes);
        var milli = Math.round((dur - seconds) * 1000.0, 0);

        minutes = ('000' + minutes).slice(-3);
        seconds = ('00' + seconds).slice(-2);
        milli = ('000' + milli).slice(-3);

        var txt = minutes + ':' + seconds + '.' + milli;
        counterElem.text(txt);
    }

    function startCounter(value) {
        recordingStart = value || now();

        if (counterInterval != null) {
            return;
        }

        counterInterval = window.setInterval(function() {
            updateCounter();
        }, counterRefresh);

        recordInterval = window.setInterval(function() {
            recordElem.toggleClass('transport-record-flash');
        }, recordRefresh);
    }

    function stopCounter() {
        clearInterval(counterInterval);
        clearInterval(recordInterval);
        counterInterval = null;
        recordInterval = null;
    }

    function transportRecordClick() {
        if (is_recording) {
            if (!window.confirm("Are you sure you want to stop recording now?")) {
                return;
            }

            $.post(BASE_URL + '/stop', function(data) {
                if (data.success) {
                    stopCounter();
                }

                doStatus();
            });
        } else {
            $.post(BASE_URL + '/record', function(data) {
                if (data.success) {
                    startCounter();
                }

                doStatus();
            });
        }
    }

    function doStatus() {
        $.getJSON(BASE_URL + '/status', function(data) {
            if (data.recording) {
                is_recording = true;
                startCounter(now() - data.recording_duration);
            } else {
                is_recording = false;
                stopCounter();
                updateCounter(true);
            }

            var i
            for (i = 0; i < data.sinks.length; i++) {
                var s = data.sinks[i];

                if (sinks[s] === undefined) {
                    createSink(s);
                } else {
                }
            }

            if (data.hostname) {
                setHostname(data.hostname);
            }
        });
    }

    function createSink(name) {
        if (name == 'audio') {
            return;
        }

        var img_url = BASE_URL + "/preview?sink=" + name + "&next=1";
        var elem = $("<div style='float: left; width: 50%' class='sink-" + name + "'><h3>" + name + "</h3> <img style='width: 100%' /></div>");

        var obj = {
            elem: elem,
        }

        sinks[name] = obj

        $("#sink-info").append(elem);

        var img = elem.find('img');
        console.log(img);

        var i = 0;

        img.on('load', function() {
            console.log("load", name);

             setTimeout(function() {
                img.attr('src', img_url + '&_=' + i);
                i += 1;
            }, 750);
        });

        img.attr('src', img_url);
    }

    $(function() {
        $('#transport-record').click(transportRecordClick);

        setInterval(doStatus, 1000);

    });
})();
