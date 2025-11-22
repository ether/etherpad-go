window.addEventListener('load', ()=>{
    const searchParams = new URLSearchParams(window.location.search)
    const form = document.getElementsByTagName('form')[0];
    const code_verifier = searchParams.get("code_challenge")
    if (code_verifier != null) {
        const input = document.createElement('input')
        input.type = 'hidden'
        input.name = 'code_challenge'
        input.value = code_verifier
        form.appendChild(input)
    }

    const method = searchParams.get('code_challenge_method')
    if (method != null) {
        const input = document.createElement('input')
        input.type = 'hidden'
        input.name = 'code_challenge_method'
        input.value = method
        form.appendChild(input)
    }

    const state = searchParams.get('state')
    if (state != null) {
        const input = document.createElement('input')
        input.type = 'hidden'
        input.name = 'state'
        input.value = state
        form.appendChild(input)
    }

    const redirect_uri = searchParams.get('redirect_uri')
    if (redirect_uri != null) {
        const input = document.createElement('input')
        input.type = 'hidden'
        input.name = 'redirect_uri'
        input.value = redirect_uri
        form.appendChild(input)
    }
})