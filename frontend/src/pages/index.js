import Head from 'next/head'
import Image from 'next/image'
import { Inter } from 'next/font/google'
import styles from '@/styles/Home.module.css'

// const inter = Inter({ subsets: ['latin'] })

export default function Home() {
  return (
    <div className="App">
      <h1>Flexatar Frontend Test</h1>
      <div className='row'>
        <button>With Webcam</button>
        <button>With Flexatar #1</button>
        <button>With Flexatar #2</button>

        <p>videos from others</p>
        <br/>
        <p>own videos: as captured and as seen by others</p>
      </div>
    </div>
  );
}
