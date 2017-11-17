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
            for (i = 0; data.sinks != null && i < data.sinks.length; i++) {
                var s = data.sinks[i];

                if (sinks[s.name] === undefined) {
                    createSink(s);
                }

                updateSinkStatus(s);
            }

            if (data.hostname) {
                setHostname(data.hostname);
            }
        });
    }

    function format_size(k) {
        var KB = 1000;
        var MB = 1000 * KB;
        var GB = 1000 * MB;

        if (k > GB) {
            k = k / GB;
            return k.toFixed(2) + ' GB';
        } else if (k > MB) {
            k = k / MB;
            return k.toFixed(1) + ' MB';
        } else if (k > KB) {
            k = k / KB;
            return k.toFixed(1) + ' KB';
        } else {
            return k + ' bytes';
        }
    }

    function setupPreview(elem, name) {
        var img_url = BASE_URL + '/preview?sink=' + name;
        var next_url = BASE_URL + '/preview?sink=' + name + '&next=1';

        function update() {
            elem.attr('src', img_url + '&_=' + new Date().getTime());
        }

        elem.on('load', function() {
            $.get(next_url, function() {
                update();
            });
        });

        update();
    }

    function createSink(sink) {
        name = sink.name;

        var html = `
                <div class='sink-status sink-status-${name}' data-sink-name='${name}'>
                    <div class='sink-name' id='sink-name-${name}'>${name}</div>
                    <div class='sink-stats' id='sink-stats-${name}'>
                        <span id='sink-stats-output-bw'></span> (<span id='sink-stats-output-total'></span> total)
                    </div>
                    <div class='sink-preview' id='sink-preview-${name}'>
                        <img class='sink-preview-img' id='sink-preview-img-${name}' />
                    </div>
                </div>
                `;

        if (name == 'audio') {
            html += "<div style='clear: both'></div>";
        }

        var elem = $(html);

        var obj = sinks[name] = {
            elem: elem,
        };

        $("#sink-info").append(elem);

        if (name != 'audio') {
            var img = elem.find('img');
            setupPreview(img, name);
        } else {
            elem.find('.sink-preview').hide();
        }
    }

    function updateSinkStatus(st)
    {
        var sink = sinks[st.name];
        if (!sink) {
            return;
        }

        sink.elem.find('#sink-stats-output-bw').text(format_size(st.bytes_in_per_second) + "/s");
        sink.elem.find('#sink-stats-output-total').text(format_size(st.bytes_in));
    }

    $(function() {
        $('#transport-record').click(transportRecordClick);

        setInterval(doStatus, 1000);

    });
})();
